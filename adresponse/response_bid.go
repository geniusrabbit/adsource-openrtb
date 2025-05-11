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
	"strings"

	openrtb "github.com/bsm/openrtb"
	"go.uber.org/zap"

	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/billing"
	"github.com/geniusrabbit/adcorelib/context/ctxlogger"
	"github.com/geniusrabbit/adcorelib/price"
)

// BidResponse represents an OpenRTB bid response with additional processing capabilities.
// It encapsulates the original OpenRTB response along with request context and derived data.
type BidResponse struct {
	context context.Context

	// Request and source information
	Req *adtype.BidRequest
	Src adtype.Source

	// BidResponse RTB record
	BidResponse openrtb.BidResponse

	bidRespBidCount int

	optimalBids []*openrtb.Bid
	ads         []adtype.ResponserItemCommon

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
	return r.Req.AuctionType
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
			if imp := r.Req.ImpressionByIDvariation(bid.ImpID); imp != nil {
				// Set default dimensions from impression if not present in bid
				if bid.W == 0 && bid.H == 0 {
					bid.W, bid.H = imp.Width, imp.Height
				}

				if imp.IsDirect() {
					// Handle direct creative content from bid extensions if not in AdMarkup
					if bid.AdMarkup == "" {
						bid.AdMarkup, _ = customDirectURL(bid.Ext)
					}
					// Handle XML-formatted pop markup
					if strings.HasPrefix(bid.AdMarkup, `<?xml`) {
						bid.AdMarkup, _ = decodePopMarkup([]byte(bid.AdMarkup))
					}
				}
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
		if imp := r.Req.ImpressionByIDvariation(bid.ImpID); imp != nil {
			if bidItem := r.prepareBidItem(bid, imp); bidItem != nil {
				r.ads = append(r.ads, bidItem)
			}
		}
	}
}

// prepareBidItem creates a standardized ResponseBidItem from an OpenRTB bid and impression.
// It handles different creative formats (direct, native, banner) and sets up pricing information.
// Returns nil if no appropriate format can be determined.
func (r *BidResponse) prepareBidItem(bid *openrtb.Bid, imp *adtype.Impression) *ResponseBidItem {
	var (
		format  *types.Format
		bidItem *ResponseBidItem
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
		// Handle direct response format (like click URLs)
		bidItem = &ResponseBidItem{
			ItemID:     imp.ID,
			Src:        r.Src,
			Req:        r.Req,
			Imp:        imp,
			Bid:        bid,
			FormatType: types.FormatDirectType,
			RespFormat: format,
			ActionLink: bid.AdMarkup,
		}
	case format.IsNative():
		// Handle native ad format with structured data
		native, err := decodeNativeMarkup([]byte(bid.AdMarkup))
		if err == nil {
			bidItem = &ResponseBidItem{
				ItemID:     imp.ID,
				Src:        r.Src,
				Req:        r.Req,
				Imp:        imp,
				Bid:        bid,
				FormatType: types.FormatNativeType,
				RespFormat: format,
				Native:     native,
				ActionLink: native.Link.URL,
				Data:       extractNativeDataFromImpression(imp, native),
			}
		} else {
			// Log native markup decoding failures
			ctxlogger.Get(r.Context()).Debug(
				"Failed to decode native markup",
				zap.String("markup", bid.AdMarkup),
				zap.Error(err),
			)
		}
	case format.IsBanner() || format.IsProxy():
		// Handle banner or proxy creative content
		bidItem = &ResponseBidItem{
			ItemID:     imp.ID,
			Src:        r.Src,
			Req:        r.Req,
			Imp:        imp,
			Bid:        bid,
			FormatType: bannerFormatType(bid.AdMarkup),
			RespFormat: format,
		}
	}

	if bidItem != nil {
		// Calculate final bid pricing based on system rules and convert to appropriate units
		bidPrice := price.CalculateNewBidPrice(billing.MoneyFloat(bid.Price/1000), bidItem)

		bidItem.PriceScope = price.PriceScopeView{
			MaxBidPrice: bidPrice,
			BidPrice:    bidPrice,
			ViewPrice:   billing.MoneyFloat(bid.Price / 1000), // Convert from micros (CPM) to actual price
			ECPM:        billing.MoneyFloat(bid.Price),        // Original eCPM price
		}
	}

	return bidItem
}

// Request returns the original bid request associated with this response.
func (r *BidResponse) Request() *adtype.BidRequest {
	return r.Req
}

// Ads returns the list of processed ad items derived from the bid response.
func (r *BidResponse) Ads() []adtype.ResponserItemCommon {
	return r.ads
}

// Item returns a specific ad item by impression ID.
// Returns nil if no matching item is found.
func (r *BidResponse) Item(impid string) adtype.ResponserItemCommon {
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
	bids := make(map[string]*openrtb.Bid, len(r.BidResponse.SeatBid))
	for _, seat := range r.BidResponse.SeatBid {
		for _, bid := range seat.Bid {
			if obid, ok := bids[bid.ImpID]; !ok || obid.Price < bid.Price {
				bids[bid.ImpID] = &bid
			}
		}
	}

	// Convert map to slice for return
	optimalBids := make([]*openrtb.Bid, 0, len(bids))
	for _, b := range bids {
		optimalBids = append(optimalBids, b)
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
		return r.Req.Ctx
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
	_ adtype.Responser = &BidResponse{}
)
