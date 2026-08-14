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
//   if err := driver.Test(request); err == nil {
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
	"github.com/geniusrabbit/adcorelib/errtype"
	"github.com/geniusrabbit/adcorelib/eventtraking/events"
	"github.com/geniusrabbit/adcorelib/eventtraking/eventstream"
	"github.com/geniusrabbit/adcorelib/fasttime"
	"github.com/geniusrabbit/adcorelib/openlatency"
	"github.com/geniusrabbit/adcorelib/openlatency/prometheuswrapper"

	"github.com/geniusrabbit/adsource-openrtb/response/requester"
	"github.com/geniusrabbit/adsource-openrtb/sources"
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
	source     *admodels.RTBSource
	sourceInfo *adtype.SourceInfo

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
	sourceInfo := sources.Sources.SourceInfoByDSPDomain(source.Domain())
	if sourceInfo != nil {
		sourceInfo.ID = gocast.Str(source.ID)
		sourceInfo.Protocol = source.Protocol
	} else {
		sourceInfo = &adtype.SourceInfo{
			ID:       gocast.Str(source.ID),
			Protocol: source.Protocol,
		}
	}
	return &driver{
		source:       source,
		rtbRequester: rtbRequester,
		sourceInfo:   sourceInfo,
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
	return d.sourceInfo
}

// AccountID of source
func (d *driver) AccountID() uint64 {
	if d.source.Account == nil {
		return 0
	}
	return d.source.Account.ID()
}

var (
	ErrNilRequest           = errtype.Error("nil bid request")
	ErrErrorCircuitOpen     = errtype.Error("error circuit open")
	ErrRPSLimitExceeded     = errtype.Error("rps limit exceeded")
	ErrTargetFilterRejected = errtype.Error("target filter rejected")
)

// Test request before processing.
// Returns a typed cause on rejection, or nil when the request may proceed.
func (d *driver) Test(request adtype.BidRequester) error {
	if request == nil {
		return ErrNilRequest
	}

	if d.source.RPS > 0 {
		if d.source.Options.ErrorsIgnore == 0 && !d.errorCounter.Next() {
			d.latencyMetrics.IncSkip()
			return ErrErrorCircuitOpen
		}

		now := fasttime.UnixTimestampNano()
		if now-atomic.LoadUint64(&d.lastRequestTime) >= uint64(time.Second) {
			atomic.StoreUint64(&d.lastRequestTime, now)
			d.rpsCurrent.Set(0)
		} else if d.rpsCurrent.Get() >= int64(d.source.RPS) {
			d.latencyMetrics.IncSkip()
			return ErrRPSLimitExceeded
		}
	}

	pointers := request.TargetPointers()
	if len(pointers) == 0 {
		return nil
	}

	var lastErr error
	for _, pointer := range pointers {
		err := d.source.Test(pointer)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	d.latencyMetrics.IncSkip()
	if lastErr != nil {
		return lastErr
	}
	return ErrTargetFilterRejected
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
func (d *driver) Bid(request adtype.BidRequester) adtype.Response {
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

	ctxl := response.Context()

	// Send win notification if NotifyWinURL is set in the bid content and the bid is a winner.
	if nurl := item.ContentItemString(adtype.ContentItemNotifyWinURL); nurl != "" {
		if prep := adtype.ContentMappingPreparer(response, item); prep != nil {
			nurl = prep.Replace(nurl)
		}
		ctxlogger.Get(ctxl).Info("ping", zap.String("url", nurl))
		err := eventstream.WinsFromContext(ctxl).Send(ctxl, nurl)
		if err != nil {
			ctxlogger.Get(ctxl).Error("ping error", zap.Error(err))
		}
	}

	// Send win event to event stream for tracking
	err := eventstream.StreamFromContext(ctxl).
		Send(events.SourceWin, events.StatusUndefined, response, item)
	if err != nil {
		ctxlogger.Get(ctxl).Error("send win event", zap.Error(err))
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
