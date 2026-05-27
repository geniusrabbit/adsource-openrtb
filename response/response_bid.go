// @project GeniusRabbit corelib 2016 - 2019, 2024 - 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2016 - 2019, 2024 - 2025
//
// Package response handles the processing and manipulation of OpenRTB bid responses.
// This file contains the BidResponse implementation which handles the preparation
// of bid responses, extraction of optimal bids, and conversion of OpenRTB bid responses
// into standardised ad response items.
//
// The BidResponse struct manages the lifecycle of OpenRTB bid responses including:
//   - Response preparation and URL/markup handling
//   - Bid validation and optimisation
//   - Format detection and response item creation
//   - Price calculation and adjustment
package response

import (
	"context"
	"fmt"
	"iter"
	"sort"
	"strings"

	openrtb "github.com/bsm/openrtb"
	"github.com/demdxx/xtypes"
	"go.uber.org/zap"

	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/context/ctxlogger"

	"github.com/geniusrabbit/adsource-openrtb/response/banner"
	"github.com/geniusrabbit/adsource-openrtb/response/direct"
	"github.com/geniusrabbit/adsource-openrtb/response/native"
	vastp "github.com/geniusrabbit/adsource-openrtb/response/vast"
)

// BidResponse represents an OpenRTB bid response with additional processing capabilities.
// It encapsulates the original OpenRTB response along with request context and derived data.
type BidResponse struct {
	context context.Context

	// Request and source information
	Req adtype.BidRequester
	Src adtype.Source

	// BidResponse RTB record
	BidResponse openrtb.BidResponse

	bidRespBidCount int

	optimalBids []*openrtb.Bid
	ads         []adtype.ResponseItemCommon

	// TODO: add errors list
}

// AuctionID returns the auction identifier from the bid response.
// This is the ID that was originally passed in the bid request.
func (r *BidResponse) AuctionID() string {
	return r.BidResponse.ID
}

// AuctionType returns the auction type from the original bid request.
func (r *BidResponse) AuctionType() types.AuctionType {
	return r.Req.AuctionType()
}

// Source returns the source of the bid response.
func (r *BidResponse) Source() adtype.Source {
	return r.Src
}

// Prepare processes the bid response to make it ready for use in ad serving.
// This includes processing bid markup and URLs, replacing macros in creative content,
// extracting optimal bids, and creating standardised ad objects.
func (r *BidResponse) Prepare() {
	r.bidRespBidCount = 0

	for i, seat := range r.BidResponse.SeatBid {
		for i, bid := range seat.Bid {
			imp := xtypes.Slice[*adtype.Impression](r.Req.Impressions()).FirstOr(nil,
				func(imp **adtype.Impression) bool { return strings.HasPrefix(bid.ImpID, (*imp).ID) })

			if imp != nil && (bid.W == 0 && bid.H == 0) {
				bid.W, bid.H = imp.Width, imp.Height
			}

			replacer := r.newBidReplacer(&bid)
			bid.AdMarkup = replacer.Replace(bid.AdMarkup)
			bid.NURL = prepareURL(bid.NURL, replacer)
			bid.BURL = prepareURL(bid.BURL, replacer)

			seat.Bid[i] = bid
		}

		r.BidResponse.SeatBid[i] = seat
		r.bidRespBidCount += len(seat.Bid)
	}

	for _, bid := range r.OptimalBids() {
		imp := xtypes.Slice[*adtype.Impression](r.Req.Impressions()).FirstOr(nil,
			func(imp **adtype.Impression) bool { return strings.HasPrefix(bid.ImpID, (*imp).ID) })
		if imp != nil {
			if bidItem := r.prepareBidItem(bid, imp); bidItem != nil {
				r.ads = append(r.ads, bidItem)
			}
		}
	}
}

// prepareBidItem creates a standardised ResponseBidItem from an OpenRTB bid and impression.
// It delegates to the appropriate format-specific constructor and returns nil when the
// format cannot be determined or the constructor returns an error.
func (r *BidResponse) prepareBidItem(bid *openrtb.Bid, imp *adtype.Impression) adtype.ResponseItemCommon {
	var (
		format  *types.Format
		bidItem adtype.ResponseItemCommon
		err     error
	)

	if imp.IsDirect() {
		format = imp.FormatByType(types.FormatDirectType)
	} else {
		for _, formatObj := range imp.Formats() {
			if bid.ImpID != imp.IDByFormat(formatObj) {
				continue
			}
			format = formatObj
			break
		}
	}

	if format == nil {
		return nil
	}

	switch {
	case format.IsDirect():
		if bidItem, err = direct.New(r.Req, r.Src, bid, imp, format); err != nil {
			ctxlogger.Get(r.Context()).Debug(
				"Failed to create direct bid item",
				zap.String("markup", bid.AdMarkup),
				zap.Error(err),
			)
			return nil
		}
	case format.IsNative():
		if bidItem, err = native.New(r.Req, r.Src, bid, imp, format); err != nil {
			ctxlogger.Get(r.Context()).Debug(
				"Failed to decode native markup",
				zap.String("markup", bid.AdMarkup),
				zap.Error(err),
			)
			return nil
		}
	case format.IsBanner() || format.IsProxy():
		if bidItem, err = banner.New(r.Req, r.Src, bid, imp, format); err != nil {
			ctxlogger.Get(r.Context()).Debug(
				"Failed to decode banner markup",
				zap.String("markup", bid.AdMarkup),
				zap.Error(err),
			)
			return nil
		}
	case format.IsVideo():
		if bidItem, err = vastp.New(r.Req, r.Src, bid, imp, format); err != nil {
			ctxlogger.Get(r.Context()).Debug(
				"Failed to decode video markup",
				zap.String("markup", bid.AdMarkup),
				zap.Error(err),
			)
			return nil
		}
	}

	return bidItem
}

// Request returns the original bid request associated with this response.
func (r *BidResponse) Request() adtype.BidRequester {
	return r.Req
}

// Ads returns the list of processed ad items derived from the bid response.
func (r *BidResponse) Ads() []adtype.ResponseItemCommon {
	return r.ads
}

// IterAds returns an iterator over the ad items in the response.
func (r *BidResponse) IterAds() iter.Seq[adtype.ResponseItem] {
	return func(yield func(adtype.ResponseItem) bool) {
		for _, it := range r.ads {
			switch itV := it.(type) {
			case nil:
			case adtype.ResponseItem:
				if !yield(itV) {
					return
				}
			case adtype.ResponseMultipleItem:
				for _, mit := range itV.Ads() {
					if !yield(mit) {
						return
					}
				}
			default:
				// do nothing
			}
		}
	}
}

// Item returns a specific ad item by impression ID.
// Returns nil if no matching item is found.
func (r *BidResponse) Item(impid string) adtype.ResponseItemCommon {
	for _, it := range r.Ads() {
		if it.ImpressionID() == impid {
			return it
		}
	}
	return nil
}

// Count returns the total number of bids in the response.
func (r *BidResponse) Count() int {
	return r.bidRespBidCount
}

// Validate checks if the response meets all requirements.
// Returns an error if the response is invalid, nil otherwise.
func (r *BidResponse) Validate() error {
	if r == nil {
		return adtype.ErrResponseEmpty
	}
	err := r.BidResponse.Validate()
	if err == nil {
		for _, seat := range r.BidResponse.SeatBid {
			if seat.Group == 1 {
				return adtype.ErrResponseInvalidGroup
			}
		}
	}
	return err
}

// Error returns the validation error, if any.
func (r *BidResponse) Error() error {
	return r.Validate()
}

// OptimalBids returns the most expensive bid for each impression.
// Results are cached after the first call.
func (r *BidResponse) OptimalBids() []*openrtb.Bid {
	if len(r.optimalBids) > 0 {
		return r.optimalBids
	}

	totalBidsCount := 0
	for _, seat := range r.BidResponse.SeatBid {
		totalBidsCount += len(seat.Bid)
	}

	allBids := make([]*openrtb.Bid, 0, totalBidsCount)
	for _, seat := range r.BidResponse.SeatBid {
		for i := range seat.Bid {
			allBids = append(allBids, &seat.Bid[i])
		}
	}

	sort.Slice(allBids, func(i, j int) bool {
		return allBids[i].ImpID < allBids[j].ImpID ||
			(allBids[i].ImpID == allBids[j].ImpID && allBids[i].Price > allBids[j].Price)
	})

	optimalBids := make([]*openrtb.Bid, 0, totalBidsCount)
	for _, imp := range r.Req.Impressions() {
		added := 0
		bidCount := max(imp.Count, 1)
		for _, bid := range allBids {
			if strings.HasPrefix(bid.ImpID, imp.ID) {
				optimalBids = append(optimalBids, bid)
				added++
			}
			if added >= bidCount {
				break
			}
		}
	}

	r.optimalBids = optimalBids
	return r.optimalBids
}

// Context gets or sets the context for this response.
func (r *BidResponse) Context(ctx ...context.Context) context.Context {
	if len(ctx) > 0 {
		r.context = ctx[0]
	}
	if r.context == nil {
		return r.Req.Context()
	}
	return r.context
}

// Get retrieves a value from the response context by key.
func (r *BidResponse) Get(key string) any {
	if r.context != nil {
		return r.context.Value(key)
	}
	return nil
}

// newBidReplacer creates a [strings.Replacer] for macro substitution in creative content and URLs.
func (r *BidResponse) newBidReplacer(bid *openrtb.Bid) *strings.Replacer {
	return strings.NewReplacer(
		"${AUCTION_AD_ID}", bid.AdID,
		"${AUCTION_ID}", r.BidResponse.ID,
		"${AUCTION_BID_ID}", r.BidResponse.BidID,
		"${AUCTION_IMP_ID}", bid.ImpID,
		"${AUCTION_PRICE}", fmt.Sprintf("%.6f", bid.Price),
		"${AUCTION_CURRENCY}", "USD",
	)
}

// Release frees resources used by the response.
// This method should be called when the response is no longer needed.
func (r *BidResponse) Release() {
	if r == nil {
		return
	}
	r.Req = nil
	r.ads = r.ads[:0]
	r.optimalBids = r.optimalBids[:0]
	r.BidResponse.SeatBid = r.BidResponse.SeatBid[:0]
	r.BidResponse.Ext = r.BidResponse.Ext[:0]
}

var (
	// Verify BidResponse implements the adtype.Response interface.
	_ adtype.Response = &BidResponse{}
)
