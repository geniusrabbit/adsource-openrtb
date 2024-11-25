package adsourceopenrtb

import (
	"time"

	"github.com/geniusrabbit/adcorelib/admodels/types"
)

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

func (opts *BidRequestRTBOptions) openNativeVer() string {
	return opts.OpenNative.Ver
}

func (opts *BidRequestRTBOptions) currencies() []string {
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
