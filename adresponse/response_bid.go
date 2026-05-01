//
// @project GeniusRabbit corelib 2016 – 2019, 2024 - 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2016 – 2019, 2024 - 2025
//
// Package adresponse handles the processing and manipulation of OpenRTB bid responses.
// This file contains the BidResponse implementation which handles the preparation
// of bid responses, extraction of optimal bids, and conversion of OpenRTB bid responses
// into standardized ad response items.
//
// The BidResponse struct manages the lifecycle of OpenRTB bid responses including:
// - Response preparation and URL/markup handling
// - Bid validation and optimization
// - Format detection and response item creation
// - Price calculation and adjustment
//

package adresponse

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
// This determines whether the auction is first-price, second-price, etc.
func (r *BidResponse) AuctionType() types.AuctionType {
	return r.Req.AuctionType()
}

// Source returns the source of the bid response (e.g., which demand partner or exchange).
func (r *BidResponse) Source() adtype.Source {
	return r.Src
}

// Prepare processes the bid response to make it ready for use in ad serving.
// This includes:
// - Processing bid markup and URLs
// - Replacing macros in creative content
// - Extracting optimal bids
// - Creating standardized ad objects
func (r *BidResponse) Prepare() {
	r.bidRespBidCount = 0

	// Prepare URLs and markup for response
	for i, seat := range r.BidResponse.SeatBid {
		for i, bid := range seat.Bid {
			imp := xtypes.Slice[*adtype.Impression](r.Req.Impressions()).FirstOr(nil,
				func(imp **adtype.Impression) bool { return strings.HasPrefix(bid.ImpID, (*imp).ID) })

			// Set default dimensions from impression if not present in bid
			if imp != nil && (bid.W == 0 && bid.H == 0) {
				bid.W, bid.H = imp.Width, imp.Height
			}

			// Replace auction-related macros in creative content and tracking URLs
			replacer := r.newBidReplacer(&bid)
			bid.AdMarkup = replacer.Replace(bid.AdMarkup)
			bid.NURL = prepareURL(bid.NURL, replacer)
			bid.BURL = prepareURL(bid.BURL, replacer)

			seat.Bid[i] = bid
		}

		r.BidResponse.SeatBid[i] = seat
		r.bidRespBidCount += len(seat.Bid)
	} // end for

	// Create response ad items from the optimal bids for each impression
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

// prepareBidItem creates a standardized ResponseBidItem from an OpenRTB bid and impression.
// It handles different creative formats (direct, native, banner) and sets up pricing information.
// Returns nil if no appropriate format can be determined.
func (r *BidResponse) prepareBidItem(bid *openrtb.Bid, imp *adtype.Impression) adtype.ResponseItemCommon {
	var (
		format  *types.Format
		bidItem adtype.ResponseItemCommon
		err     error
	)

	// Determine the appropriate format based on impression type
	if imp.IsDirect() {
		format = imp.FormatByType(types.FormatDirectType)
	} else {
		// Match the bid impression ID with the correct format
		for _, formatObj := range imp.Formats() {
			if bid.ImpID != imp.IDByFormat(formatObj) {
				continue
			}
			format = formatObj
			break
		}
	}

	// No matching format found, can't create bid item
	if format == nil {
		return nil
	}

	// Create appropriate bid item based on format type
	switch {
	case format.IsDirect():
		if bidItem, err = newResponseDirectBidItem(r.Req, r.Src, bid, imp, format); err != nil {
			// Log direct bid item creation failures
			ctxlogger.Get(r.Context()).Debug(
				"Failed to create direct bid item",
				zap.String("markup", bid.AdMarkup),
				zap.Error(err),
			)
		}
	case format.IsNative():
		if bidItem, err = newResponseNativeBidItem(r.Req, r.Src, bid, imp, format); err != nil {
			// Log native markup decoding failures
			ctxlogger.Get(r.Context()).Debug(
				"Failed to decode native markup",
				zap.String("markup", bid.AdMarkup),
				zap.Error(err),
			)
		}
	case format.IsBanner() || format.IsProxy():
		if bidItem, err = newResponseBannerBidItem(r.Req, r.Src, bid, imp, format); err != nil {
			// Log banner markup decoding failures
			ctxlogger.Get(r.Context()).Debug(
				"Failed to decode banner markup",
				zap.String("markup", bid.AdMarkup),
				zap.Error(err),
			)
		}
	case format.IsVideo():
		if bidItem, err = newResponseVASTBidItem(r.Req, r.Src, bid, imp, format); err != nil {
			// Log video markup decoding failures
			ctxlogger.Get(r.Context()).Debug(
				"Failed to decode video markup",
				zap.String("markup", bid.AdMarkup),
				zap.Error(err),
			)
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
		// Check for invalid group flag (OpenRTB spec requires group=0 or unspecified)
		for _, seat := range r.BidResponse.SeatBid {
			if seat.Group == 1 {
				return adtype.ErrResponseInvalidGroup
			}
		}
	}
	return err
}

// Error returns the validation error, if any.
// This is a convenience method that calls Validate().
func (r *BidResponse) Error() error {
	return r.Validate()
}

// OptimalBids returns the most expensive bid for each impression.
// Results are cached after first call for performance.
func (r *BidResponse) OptimalBids() []*openrtb.Bid {
	if len(r.optimalBids) > 0 {
		return r.optimalBids
	}

	// Find the highest-priced bid for each impression ID
	totalBidsCount := 0

	// Sort bids by impression ID and price to ensure we get the most expensive bid for each impression
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

	// Map to store the highest bid for each impression ID
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
// If a context is provided, it will be stored. If not, the current context
// or request context is returned.
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
// Returns nil if context is nil or key is not found.
func (r *BidResponse) Get(key string) any {
	if r.context != nil {
		return r.context.Value(key)
	}
	return nil
}

// newBidReplacer creates a string replacer for macro substitution in creative content and URLs.
// It handles standard OpenRTB macros for auction IDs, prices, etc.
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
	// Verify BidResponse implements the adtype.Responser interface
	_ adtype.Response = &BidResponse{}
)
