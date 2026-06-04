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
//   request := &bidrequest.BidRequest{ /* initialize bid request */ }
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
	"context"
	"slices"
	"sync/atomic"
	"time"

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
	"github.com/geniusrabbit/adcorelib/openlatency"
	"github.com/geniusrabbit/adcorelib/openlatency/prometheuswrapper"

	"github.com/geniusrabbit/adsource-openrtb/response/requester"
)

const (
	defaultMinWeight = 0.001
)

// RTBRequester wraps the execution of a single RTB server round-trip.
// It is re-exported from response/requester for convenience.
type RTBRequester = requester.RTBRequester

type driver struct {
	lastRequestTime uint64

	// Requests RPS counter
	rpsCurrent     counter.Counter
	errorCounter   counter.ErrorCounter
	latencyMetrics *prometheuswrapper.Wrapper

	// Original source model
	source *admodels.RTBSource

	// RTB source requester (performs actual HTTP call)
	rtbRequester RTBRequester
}

func newDriver(_ context.Context, source *admodels.RTBSource, rtbRequester RTBRequester, _ ...any) (*driver, error) {
	if source == nil {
		return nil, ErrNilSource
	}
	if rtbRequester == nil {
		return nil, ErrNilHTTPClient
	}
	source.MinimalWeight = max(source.MinimalWeight, defaultMinWeight)
	return &driver{
		source:       source,
		rtbRequester: rtbRequester,
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

// Info returns information about the source platform and the source protocol
func (d *driver) Info() *adtype.SourceInfo {
	return &adtype.SourceInfo{
		ID:       gocast.Str(d.source.ID),
		Protocol: d.source.Protocol,
	}
}

// AccountID of source
func (d *driver) AccountID() uint64 {
	if d.source.Account == nil {
		return 0
	}
	return d.source.Account.ID()
}

// Test request before processing
func (d *driver) Test(request adtype.BidRequester) bool {
	if request == nil {
		return false
	}

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

	if !slices.ContainsFunc(request.TargetPointers(), d.source.Test) {
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
func (d *driver) Bid(request adtype.BidRequester) (response adtype.Response) {
	beginTime := fasttime.UnixTimestampNano()
	d.rpsCurrent.Inc(1)
	d.latencyMetrics.BeginQuery()

	// Send request to source and get response
	response, err := d.rtbRequester.Request(request, beginTime)
	if err != nil {
		if errors.Is(err, ErrResponseNoBid) {
			// No bid is not an error, so we just return empty response
			response = bidresponse.NewEmptyResponse(request, d, err)
		} else {
			response = adtype.NewErrorResponse(request, err)
			ctxlogger.Get(request.Context()).Error("bid", zap.Error(err))
		}
	}

	// Update metrics based on response
	// Success if there are ads in the response and no error; NoBid if no ads but also no error; otherwise, it's an error case
	if response != nil && response.Error() == nil {
		if len(response.Ads()) > 0 {
			d.latencyMetrics.IncSuccess()
		} else {
			d.latencyMetrics.IncNobid()
		}
	}

	if response == nil {
		response = bidresponse.NewEmptyResponse(request, d, err)
	}
	return response
}

// ProcessResponseItem result or error
func (d *driver) ProcessResponseItem(response adtype.Response, item adtype.ResponseItem) {
	if response == nil || response.Error() != nil {
		return
	}
	for _, ad := range response.Ads() {
		switch bid := ad.(type) {
		case adtype.ResponseItem:
			if bid.Source().ID() != d.ID() {
				ctxlogger.Get(response.Context()).Debug("bid source mismatch",
					zap.Uint64("source_id", bid.Source().ID()),
					zap.Uint64("driver_id", d.ID()),
				)
				continue
			}
			if nurl := bid.ContentItemString(adtype.ContentItemNotifyDisplayURL); nurl != "" {
				ctxlogger.Get(response.Context()).Info("ping", zap.String("url", nurl))
				err := eventstream.WinsFromContext(response.Context()).Send(response.Context(), nurl)
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
