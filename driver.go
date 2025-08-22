// Package openrtb facilitates the interaction with the OpenRTB (Real-Time Bidding) protocol,
// enabling real-time bidding requests and responses following OpenRTB standards.
//
// Features:
// - Request and Response Handling: Manages bid requests and responses for OpenRTB 2.x and 3.x.
// - Metrics and Logging: Integrates comprehensive metrics and logging using zap and prometheuswrapper.
// - Error Handling: Implements robust error handling and retry mechanisms.
// - Customizable Headers: Allows customization of HTTP request headers.
// - Rate Limiting: Supports RPS (Requests Per Second) limits to control request rates.
//
// The main component of the package is the `driver` struct which handles the lifecycle of a bid request,
// including preparation, execution, and response processing. It utilizes various supporting packages for
// logging, metrics, and HTTP client functionalities.
//
// Usage:
//
// Initialization:
//   ctx := context.Background()
//   source := &admodels.RTBSource{ /* initialize with source details */ }
//   netClient := httpclient.New() // Or your custom HTTP client
//
//   driver, err := newDriver(ctx, source, netClient)
//   if err != nil {
//       // handle error
//   }
//
// Sending a Bid Request:
//   request := &adtype.BidRequest{ /* initialize bid request */ }
//   if driver.Test(request) {
//       response := driver.Bid(request)
//       // process response
//   }
//
// Handling Metrics:
//   metrics := driver.Metrics()
//   // log or process metrics
//
// Functions:
// - newDriver: Initializes a new driver instance.
// - ID: Returns the ID of the source.
// - Protocol: Returns the protocol version.
// - Test: Tests if the request meets the criteria for processing.
// - PriceCorrectionReduceFactor: Returns the price correction reduce factor.
// - RequestStrategy: Returns the request strategy.
// - Bid: Processes a bid request and returns a response.
// - ProcessResponseItem: Processes individual response items.
// - RevenueShareReduceFactor: Returns the revenue share reduce factor.
// - Metrics: Returns platform

package adsourceopenrtb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bsm/openrtb"
	"github.com/demdxx/gocast/v2"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/adquery/bidresponse"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/context/ctxlogger"
	counter "github.com/geniusrabbit/adcorelib/errorcounter"
	"github.com/geniusrabbit/adcorelib/eventtraking/events"
	"github.com/geniusrabbit/adcorelib/eventtraking/eventstream"
	"github.com/geniusrabbit/adcorelib/fasttime"
	"github.com/geniusrabbit/adcorelib/net/httpclient"
	"github.com/geniusrabbit/adcorelib/openlatency"
	"github.com/geniusrabbit/adcorelib/openlatency/prometheuswrapper"

	"github.com/geniusrabbit/adsource-openrtb/adresponse"
)

const (
	headerRequestOpenRTBVersion  = "X-Openrtb-Version"
	headerRequestOpenRTBVersion2 = "2.5"
	headerRequestOpenRTBVersion3 = "3.0"
	defaultMinWeight             = 0.001
)

type driver struct {
	lastRequestTime uint64

	// Requests RPS counter
	rpsCurrent     counter.Counter
	errorCounter   counter.ErrorCounter
	latencyMetrics *prometheuswrapper.Wrapper

	// Original source model
	source *admodels.RTBSource

	// Request headers
	headers map[string]string

	// Client of HTTP requests
	netClient httpclient.Driver
}

func newDriver(_ context.Context, source *admodels.RTBSource, netClient httpclient.Driver, _ ...any) (*driver, error) {
	source.MinimalWeight = max(source.MinimalWeight, defaultMinWeight)
	return &driver{
		source:    source,
		headers:   source.Headers.DataOr(nil),
		netClient: netClient,
		latencyMetrics: prometheuswrapper.NewWrapperDefault("adsource_",
			[]string{"id", "protocol", "driver"},
			[]string{gocast.Str(source.ID), source.Protocol, "openrtb"},
		),
	}, nil
}

// ID of source
func (d *driver) ID() uint64 { return d.source.ID }

// ObjectKey of source
func (d *driver) ObjectKey() uint64 { return d.source.ID }

// Protocol of source
func (d *driver) Protocol() string { return d.source.Protocol }

// Test request before processing
func (d *driver) Test(request *adtype.BidRequest) bool {
	if d.source.RPS > 0 {
		if d.source.Options.ErrorsIgnore == 0 && !d.errorCounter.Next() {
			d.latencyMetrics.IncSkip()
			return false
		}

		now := fasttime.UnixTimestampNano()
		if now-atomic.LoadUint64(&d.lastRequestTime) >= uint64(time.Second) {
			atomic.StoreUint64(&d.lastRequestTime, now)
			d.rpsCurrent.Set(0)
		} else if d.rpsCurrent.Get() >= int64(d.source.RPS) {
			d.latencyMetrics.IncSkip()
			return false
		}
	}

	if !d.source.Test(request) {
		d.latencyMetrics.IncSkip()
		return false
	}

	return true
}

// PriceCorrectionReduceFactor which is a potential
// Returns percent from 0 to 1 for reducing of the value
// If there is 10% of price correction, it means that 10% of the final price must be ignored
func (d *driver) PriceCorrectionReduceFactor() float64 {
	return d.source.PriceCorrectionReduceFactor()
}

// RequestStrategy description
func (d *driver) RequestStrategy() adtype.RequestStrategy {
	return adtype.AsynchronousRequestStrategy
}

// Bid request for standart system filter
func (d *driver) Bid(request *adtype.BidRequest) (response adtype.Responser) {
	beginTime := fasttime.UnixTimestampNano()
	d.rpsCurrent.Inc(1)
	d.latencyMetrics.BeginQuery()

	httpRequest, err := d.request(request)
	if err != nil {
		return adtype.NewErrorResponse(request, err)
	}

	resp, err := d.netClient.Do(httpRequest)
	d.latencyMetrics.UpdateQueryLatency(time.Duration(fasttime.UnixTimestampNano() - beginTime))

	if err != nil {
		d.processHTTPReponse(resp, err)
		ctxlogger.Get(request.Ctx).Debug("bid",
			zap.String("source_url", d.source.URL),
			zap.Error(err))
		return adtype.NewErrorResponse(request, err)
	}

	ctxlogger.Get(request.Ctx).Debug("bid",
		zap.String("source_url", d.source.URL),
		zap.String("http_response_status_txt", http.StatusText(resp.StatusCode())),
		zap.Int("http_response_status", resp.StatusCode()))

	if resp.StatusCode() == http.StatusNoContent {
		d.latencyMetrics.IncNobid()
		return adtype.NewErrorResponse(request, ErrNoCampaignsStatus)
	}

	if resp.StatusCode() != http.StatusOK {
		d.processHTTPReponse(resp, nil)
		return adtype.NewErrorResponse(request, ErrInvalidResponseStatus)
	}

	defer resp.Close()
	if res, err := d.unmarshal(request, resp.Body()); d.source.Options.Trace != 0 && err != nil {
		response = adtype.NewErrorResponse(request, err)
		ctxlogger.Get(request.Ctx).Error("bid response", zap.Error(err))
	} else if res != nil {
		response = res
	}

	if response != nil && response.Error() == nil {
		if len(response.Ads()) > 0 {
			d.latencyMetrics.IncSuccess()
		} else {
			d.latencyMetrics.IncNobid()
		}
	}

	d.processHTTPReponse(resp, err)
	if response == nil {
		response = bidresponse.NewEmptyResponse(request, d, err)
	}
	return response
}

// ProcessResponseItem result or error
func (d *driver) ProcessResponseItem(response adtype.Responser, item adtype.ResponserItem) {
	if response == nil || response.Error() != nil {
		return
	}
	for _, ad := range response.Ads() {
		switch bid := ad.(type) {
		case *adresponse.ResponseBidItem:
			if bid.Source().ID() != d.ID() {
				ctxlogger.Get(response.Context()).Debug("bid source mismatch",
					zap.Uint64("source_id", bid.Source().ID()),
					zap.Uint64("driver_id", d.ID()),
				)
				continue
			}
			if len(bid.Bid.NURL) > 0 {
				ctxlogger.Get(response.Context()).Info("ping", zap.String("url", bid.Bid.NURL))
				err := eventstream.WinsFromContext(response.Context()).Send(response.Context(), bid.Bid.NURL)
				if err != nil {
					ctxlogger.Get(response.Context()).Error("ping error", zap.Error(err))
				}
			}
			err := eventstream.StreamFromContext(response.Context()).
				Send(events.SourceWin, events.StatusUndefined, response, bid)
			if err != nil {
				ctxlogger.Get(response.Context()).Error("send win event", zap.Error(err))
			}
		default:
			// Dummy...
		}
	}
}

// Weight of the source
func (d *driver) Weight() float64 {
	return d.source.MinimalWeight
}

///////////////////////////////////////////////////////////////////////////////
/// Implementation of platform.Metrics interface
///////////////////////////////////////////////////////////////////////////////

// Metrics information of the platform
func (d *driver) Metrics() *openlatency.MetricsInfo {
	var info openlatency.MetricsInfo
	d.latencyMetrics.FillMetrics(&info)
	info.ID = d.ID()
	info.Protocol = d.source.Protocol
	info.QPSLimit = d.source.RPS
	return &info
}

///////////////////////////////////////////////////////////////////////////////
/// Internal methods
///////////////////////////////////////////////////////////////////////////////

// prepare request for RTB
func (d *driver) request(request *adtype.BidRequest) (req httpclient.Request, err error) {
	var (
		rtbRequest interface{ Validate() error }
		bufData    bytes.Buffer
	)

	if d.source.Protocol == "openrtb3" {
		rtbRequest = requestToRTBv3(request, d.getRequestOptions()...)
	} else {
		rtbRequest = requestToRTBv2(request, d.getRequestOptions()...)
	}

	if d.source.Options.Trace != 0 {
		ctxlogger.Get(request.Ctx).Error("trace marshal", zap.String("src_url", d.source.URL))
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rtbRequest)
	}

	if err := rtbRequest.Validate(); err != nil {
		return nil,
			errors.Wrap(err, fmt.Sprintf("source[%s]: %d", d.source.Protocol, d.source.ID))
	}

	// Prepare data for request
	if err = json.NewEncoder(&bufData).Encode(rtbRequest); err != nil {
		return nil,
			errors.Wrap(err, fmt.Sprintf("source[%s]: %d", d.source.Protocol, d.source.ID))
	}

	// Create new request
	if req, err = d.netClient.Request(d.source.Method, d.source.URL, &bufData); err != nil {
		return req, err
	}

	d.fillRequest(request, req)
	return req, nil
}

func (d *driver) unmarshal(request *adtype.BidRequest, r io.Reader) (_ *adresponse.BidResponse, err error) {
	var bidResp openrtb.BidResponse

	switch d.source.RequestType {
	case RequestTypeJSON:
		if d.source.Options.Trace != 0 {
			var data []byte
			if data, err = io.ReadAll(r); err == nil {
				var buf bytes.Buffer
				_ = json.Indent(&buf, data, "", "  ")
				ctxlogger.Get(request.Ctx).Error("trace unmarshal",
					zap.String("src_url", d.source.URL))
				fmt.Fprintln(os.Stdout, "UNMARSHAL: "+buf.String())
				err = json.Unmarshal(data, &bidResp)
			}
		} else {
			err = json.NewDecoder(r).Decode(&bidResp)
		}
	case RequestTypeXML, RequestTypeProtobuff:
		err = fmt.Errorf("request body type not supported: %s", d.source.RequestType.Name())
	default:
		err = fmt.Errorf("undefined request type: %s", d.source.RequestType.Name())
	}

	if err != nil {
		return nil, err
	}

	// Check response for support HTTPS
	if request.IsSecure() {
		for _, seat := range bidResp.SeatBid {
			for _, bid := range seat.Bid {
				if strings.Contains(bid.AdMarkup, "http://") {
					return nil, ErrResponseAreNotSecure
				}
			}
		} // end for
	}

	// Check response for price limits
	if d.source.MaxBid > 0 {
		maxBid := d.source.MaxBid.Float64()
		for i, seat := range bidResp.SeatBid {
			changed := false
			for j, bid := range seat.Bid {
				if bid.Price > maxBid {
					// Remove bid from response if price is more than max bid
					// TODO: add metrics for this case
					seat.Bid = append(seat.Bid[:j], seat.Bid[j+1:]...)
					changed = true
				}
			}
			if changed {
				if len(seat.Bid) == 0 {
					bidResp.SeatBid = append(bidResp.SeatBid[:i], bidResp.SeatBid[i+1:]...)
				} else {
					bidResp.SeatBid[i] = seat
				}
			}
		}
	}

	// If the response is empty, then return nil
	if len(bidResp.SeatBid) == 0 {
		return nil, nil
	}

	// Build response
	bidResponse := &adresponse.BidResponse{
		Src:         d,
		Req:         request,
		BidResponse: bidResp,
	}

	bidResponse.Prepare()
	return bidResponse, nil
}

// fillRequest of HTTP
func (d *driver) fillRequest(request *adtype.BidRequest, httpReq httpclient.Request) {
	httpReq.SetHeader("Content-Type", "application/json")

	// Set OpenRTB version
	if _, ok := d.headers[headerRequestOpenRTBVersion]; !ok {
		if d.source.Protocol == "openrtb3" {
			httpReq.SetHeader(headerRequestOpenRTBVersion, headerRequestOpenRTBVersion3)
		} else {
			httpReq.SetHeader(headerRequestOpenRTBVersion, headerRequestOpenRTBVersion2)
		}
	}

	// Set request timemark for latency tracking
	httpReq.SetHeader(openlatency.HTTPHeaderRequestTimemark,
		strconv.FormatInt(openlatency.RequestInitTime(request.Time()), 10))

	// Fill default headers
	for key, value := range d.headers {
		httpReq.SetHeader(key, value)
	}
}

// @link https://golang.org/src/net/http/status.go
func (d *driver) processHTTPReponse(resp httpclient.Response, err error) {
	switch {
	case err != nil || resp == nil ||
		(resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent):
		if errors.Is(err, http.ErrHandlerTimeout) {
			d.latencyMetrics.IncTimeout()
		}
		d.errorCounter.Inc()
		if resp == nil {
			d.latencyMetrics.IncError(openlatency.MetricErrorHTTP, "")
		} else {
			d.latencyMetrics.IncError(openlatency.MetricErrorHTTP, http.StatusText(resp.StatusCode()))
		}
	default:
		d.errorCounter.Dec()
	}
}

func (d *driver) getRequestOptions() []BidRequestRTBOption {
	return []BidRequestRTBOption{
		WithRTBOpenNativeVersion("1.1"),
		WithFormatFilter(d.source.TestFormat),
		WithMaxTimeDuration(time.Duration(d.source.Timeout) * time.Millisecond),
		WithAuctionType(d.source.AuctionType),
		WithBidFloor(d.source.MinBid.Float64()),
	}
}
