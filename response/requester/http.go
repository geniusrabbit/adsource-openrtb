package requester

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bsm/openrtb"
	"github.com/demdxx/gocast/v2"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/context/ctxlogger"
	counter "github.com/geniusrabbit/adcorelib/errorcounter"
	"github.com/geniusrabbit/adcorelib/fasttime"
	"github.com/geniusrabbit/adcorelib/net/httpclient"
	"github.com/geniusrabbit/adcorelib/openlatency"
	"github.com/geniusrabbit/adcorelib/openlatency/prometheuswrapper"

	requestoptions "github.com/geniusrabbit/adsource-openrtb/request/options"
	"github.com/geniusrabbit/adsource-openrtb/request/rtbrules"
	requestv2 "github.com/geniusrabbit/adsource-openrtb/request/v2"
	requestv3 "github.com/geniusrabbit/adsource-openrtb/request/v3"
	response "github.com/geniusrabbit/adsource-openrtb/response"
	"github.com/geniusrabbit/adsource-openrtb/rules"
)

const (
	headerRequestOpenRTBVersion  = "X-Openrtb-Version"
	headerRequestOpenRTBVersion2 = "2.5"
	headerRequestOpenRTBVersion3 = "3.0"
)

// HttpRTBRequester implements RTBRequester by performing real HTTP calls to an RTB source.
type HttpRTBRequester struct {
	// Source interface used to tag bid response items
	src adtype.Source

	// Original source model
	source *admodels.RTBSource

	// Request headers
	headers map[string]string

	// Client of HTTP requests
	netClient httpclient.Driver

	// Request builder (v2 or v3 depending on source.Protocol)
	builder requestoptions.RequestBuilder
	rules   rtbrules.RTBRuler

	// Metrics and error tracking
	latencyMetrics *prometheuswrapper.Wrapper
	errorCounter   counter.ErrorCounter
}

// NewHttpRTBRequester creates a new HttpRTBRequester for the given source and HTTP client.
func NewHttpRTBRequester(source *admodels.RTBSource, netClient httpclient.Driver) (*HttpRTBRequester, error) {
	if source == nil {
		return nil, ErrNilSource
	}
	if netClient == nil {
		return nil, ErrNilHTTPClient
	}

	formatChecker := func(format *types.Format, isInterstitial bool) bool {
		if isInterstitial {
			if len(source.Filter.InterstitialFormats) == 0 {
				return source.Filter.TestFormat(format)
			}
			return source.Filter.TestInterstitialFormat(format)
		}
		return source.Filter.TestFormat(format)
	}

	var (
		builder  requestoptions.RequestBuilder
		rulesObj *rtbrules.RTBRules
		// ruler is the interface-typed handle for the builder constructors.
		// We must not assign rulesObj directly when it is nil: a (*RTBRules)(nil)
		// assigned to an RTBRuler interface produces a non-nil interface value,
		// which defeats the "if rules != nil" guards inside the builders.
		ruler rtbrules.RTBRuler
	)

	if source.Config.Rules != "" {
		if rulesObj = rules.Rules[source.Config.Rules]; rulesObj == nil {
			return nil, fmt.Errorf("source[%s]: %d: rules %s not found", source.Protocol, source.ID, source.Config.Rules)
		}
		ruler = rulesObj // only assign to the interface when concrete value is non-nil
	}

	if source.Protocol == "openrtb3" {
		builder = requestv3.New(formatChecker, ruler)
	} else {
		builder = requestv2.New(formatChecker, ruler)
	}

	return &HttpRTBRequester{
		source:    source,
		headers:   source.Headers.DataOr(nil),
		netClient: netClient,
		builder:   builder,
		rules:     ruler,
		latencyMetrics: prometheuswrapper.NewWrapperDefault("adsource_",
			[]string{"id", "protocol", "driver"},
			[]string{gocast.Str(source.ID), source.Protocol, "openrtb"},
		),
	}, nil
}

// SetSource sets the adtype.Source used to tag bid response items.
// Call this after the driver (which implements adtype.Source) has been created.
func (r *HttpRTBRequester) SetSource(src adtype.Source) {
	r.src = src
}

// Request sends the bid request to the RTB source and returns the response.
func (r *HttpRTBRequester) Request(request adtype.BidRequester, beginTime uint64) (adtype.Response, error) {
	var (
		resp             adtype.Response
		httpRequest, err = r.buildRequest(request)
	)
	if err != nil {
		return nil, err
	}

	// Send request to source
	httpResp, err := r.netClient.Do(httpRequest)
	r.latencyMetrics.UpdateQueryLatency(time.Duration(fasttime.UnixTimestampNano() - beginTime))

	// Process response status and errors
	if err != nil {
		r.processHTTPResponse(httpResp, err)
		ctxlogger.Get(request.Context()).Debug("bid",
			zap.String("source_url", r.source.URL),
			zap.Error(err))
		return nil, err
	}
	defer func() { _ = httpResp.Close() }()

	// Log response status and latency
	ctxlogger.Get(request.Context()).Debug("bid",
		zap.String("source_url", r.source.URL),
		zap.String("http_response_status_txt", http.StatusText(httpResp.StatusCode())),
		zap.Int("http_response_status", httpResp.StatusCode()))

	// NOTE: StatusNoContent is the standard OpenRTB response for no bid,
	// but some sources can return StatusNotFound in this case.
	if httpResp.StatusCode() == http.StatusNoContent || httpResp.StatusCode() == http.StatusNotFound {
		r.processHTTPResponse(httpResp, nil)
		r.latencyMetrics.IncNobid()
		return nil, ErrResponseNoBid
	}

	// Not success status code
	if httpResp.StatusCode() != http.StatusOK {
		r.processHTTPResponse(httpResp, nil)
		return nil, &HTTPStatusError{Code: httpResp.StatusCode()}
	}

	// Decode response body
	res, decErr := r.unmarshal(request, httpResp.Body())
	if r.source.Options.Trace != 0 && decErr != nil {
		ctxlogger.Get(request.Context()).Error("bid response", zap.Error(decErr))
	} else if res != nil {
		resp = res
	}

	// Process response status and errors
	r.processHTTPResponse(httpResp, decErr)

	return resp, nil
}

// buildRequest prepares the HTTP request for the RTB source.
func (r *HttpRTBRequester) buildRequest(request adtype.BidRequester) (req httpclient.Request, err error) {
	var (
		rtbRequest requestoptions.RTBRequest
		bufData    bytes.Buffer
	)

	rtbRequest, err = r.builder.Build(request, r.getRequestOptions()...)
	if err != nil {
		return nil, err
	}

	if r.source.Options.Trace != 0 {
		ctxlogger.Get(request.Context()).Error("trace marshal",
			zap.String("src_url", r.source.URL))
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rtbRequest)
	}

	if err = rtbRequest.Validate(); err != nil {
		return nil,
			errors.Wrap(err, fmt.Sprintf("source[%s]: %d", r.source.Protocol, r.source.ID))
	}

	// Prepare data for request
	if err = json.NewEncoder(&bufData).Encode(rtbRequest); err != nil {
		return nil,
			errors.Wrap(err, fmt.Sprintf("source[%s]: %d", r.source.Protocol, r.source.ID))
	}

	// Create new request
	if req, err = r.netClient.Request(r.source.Method, r.source.URL, &bufData); err != nil {
		return req, err
	}

	r.fillRequest(request, req)
	return req, nil
}

func (r *HttpRTBRequester) unmarshal(request adtype.BidRequester, body io.Reader) (_ *response.BidResponse, err error) {
	var bidResp openrtb.BidResponse

	switch r.source.RequestType {
	case admodels.RTBRequestTypeJSON:
		if r.source.Options.Trace != 0 {
			var data []byte
			if data, err = io.ReadAll(body); err == nil {
				var buf bytes.Buffer
				_ = json.Indent(&buf, data, "", "  ")
				ctxlogger.Get(request.Context()).Error("trace unmarshal",
					zap.String("src_url", r.source.URL))
				_, _ = fmt.Fprintln(os.Stdout, "UNMARSHAL: "+buf.String())
				err = json.Unmarshal(data, &bidResp)
			}
		} else {
			err = json.NewDecoder(body).Decode(&bidResp)
		}
	case admodels.RTBRequestTypeXML, admodels.RTBRequestTypeProtoBUFF:
		err = &UnsupportedTypeError{TypeName: r.source.RequestType.Name()}
	default:
		err = &UndefinedTypeError{TypeName: r.source.RequestType.Name()}
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
		}
	}

	// Check response for price limits
	if r.source.MaxBid > 0 {
		maxBid := r.source.MaxBid.Float64()
		for i, seat := range bidResp.SeatBid {
			changed := false
			for j, bid := range seat.Bid {
				if bid.Price > maxBid {
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

	// If the response is empty, return nil
	if len(bidResp.SeatBid) == 0 {
		return nil, nil
	}

	// Build response
	bidResponse := &response.BidResponse{
		Src:         r.src,
		Req:         request,
		BidResponse: bidResp,
	}

	bidResponse.Prepare(r.rules)
	return bidResponse, nil
}

// fillRequest sets HTTP headers on the request.
func (r *HttpRTBRequester) fillRequest(request adtype.BidRequester, httpReq httpclient.Request) {
	httpReq.SetHeader("Content-Type", "application/json")

	// Set OpenRTB version
	if _, ok := r.headers[headerRequestOpenRTBVersion]; !ok {
		if r.source.Protocol == "openrtb3" {
			httpReq.SetHeader(headerRequestOpenRTBVersion, headerRequestOpenRTBVersion3)
		} else {
			httpReq.SetHeader(headerRequestOpenRTBVersion, headerRequestOpenRTBVersion2)
		}
	}

	// Set request timemark for latency tracking
	httpReq.SetHeader(openlatency.HTTPHeaderRequestTimemark,
		strconv.FormatInt(openlatency.RequestInitTime(request.Time()), 10))

	// Fill default headers
	for key, value := range r.headers {
		httpReq.SetHeader(key, value)
	}
}

// @link https://golang.org/src/net/http/status.go
func (r *HttpRTBRequester) processHTTPResponse(resp httpclient.Response, err error) {
	switch {
	case err != nil || resp == nil ||
		(resp.StatusCode() != http.StatusOK &&
			resp.StatusCode() != http.StatusNoContent &&
			resp.StatusCode() != http.StatusNotFound):
		if errors.Is(err, http.ErrHandlerTimeout) {
			r.latencyMetrics.IncTimeout()
		}
		r.errorCounter.Inc()
		if resp == nil {
			r.latencyMetrics.IncError(openlatency.MetricErrorHTTP, "")
		} else {
			r.latencyMetrics.IncError(openlatency.MetricErrorHTTP, http.StatusText(resp.StatusCode()))
		}
	default:
		r.errorCounter.Dec()
	}
}

func (r *HttpRTBRequester) getRequestOptions() []requestoptions.BidRequestRTBOption {
	return []requestoptions.BidRequestRTBOption{
		requestoptions.WithRTBOpenNativeVersion("1.1"),
		requestoptions.WithFormatFilter(r.source.TestFormat),
		requestoptions.WithMaxTimeDuration(time.Duration(r.source.Timeout) * time.Millisecond),
		requestoptions.WithAuctionType(r.source.AuctionType),
		requestoptions.WithBidFloor(r.source.MinBid.Float64()),
	}
}
