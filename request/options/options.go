package requestoptions

import (
	"time"

	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
)

// RTBRequest is the minimal interface shared by both OpenRTB v2 and v3 bid-request types.
type RTBRequest interface {
	Validate() error
}

// RequestBuilder constructs a protocol-specific RTB bid request from an adtype.BidRequester.
type RequestBuilder interface {
	Build(req adtype.BidRequester, opts ...BidRequestRTBOption) (RTBRequest, error)
}

// BidRequestRTBOptions of request build
type BidRequestRTBOptions struct {
	OpenNative struct {
		Ver string
	}
	FormatFilter func(f *types.Format) bool
	Currency     []string
	TimeMax      time.Duration
	AuctionType  types.AuctionType
	BidFloor     float64
}

func (opts *BidRequestRTBOptions) OpenNativeVer() string {
	return opts.OpenNative.Ver
}

func (opts *BidRequestRTBOptions) Currencies() []string {
	if len(opts.Currency) > 0 {
		return opts.Currency
	}
	return []string{"USD"}
}

// BidRequestRTBOption set function
type BidRequestRTBOption func(opts *BidRequestRTBOptions)

// WithRTBOpenNativeVersion set version
func WithRTBOpenNativeVersion(ver string) BidRequestRTBOption {
	return func(opts *BidRequestRTBOptions) {
		opts.OpenNative.Ver = ver
	}
}

// WithFormatFilter set custom method
func WithFormatFilter(f func(f *types.Format) bool) BidRequestRTBOption {
	return func(opts *BidRequestRTBOptions) {
		opts.FormatFilter = f
	}
}

// WithMaxTimeDuration of the request
func WithMaxTimeDuration(duration time.Duration) BidRequestRTBOption {
	return func(opts *BidRequestRTBOptions) {
		opts.TimeMax = duration
	}
}

// WithAuctionType set type of auction
func WithAuctionType(auction types.AuctionType) BidRequestRTBOption {
	return func(opts *BidRequestRTBOptions) {
		opts.AuctionType = auction
	}
}

// WithBidFloor set minimal bid value
func WithBidFloor(bidFloor float64) BidRequestRTBOption {
	return func(opts *BidRequestRTBOptions) {
		opts.BidFloor = max(bidFloor, 0)
	}
}
