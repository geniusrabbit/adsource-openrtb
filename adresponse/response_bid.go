//
// @project GeniusRabbit corelib 2016 – 2019, 2024
// @author Dmitry Ponomarev <demdxx@gmail.com> 2016 – 2019, 2024
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
)

// BidResponse RTB record
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

// AuctionID response
func (r *BidResponse) AuctionID() string {
	return r.BidResponse.ID
}

// AuctionType of request
func (r *BidResponse) AuctionType() types.AuctionType {
	return r.Req.AuctionType
}

// Source of response
func (r *BidResponse) Source() adtype.Source {
	return r.Src
}

// Prepare bid response
func (r *BidResponse) Prepare() {
	r.bidRespBidCount = 0

	// Prepare URLs and markup for response
	for i, seat := range r.BidResponse.SeatBid {
		for i, bid := range seat.Bid {
			if imp := r.Req.ImpressionByIDvariation(bid.ImpID); imp != nil {
				// Prepare date for bid W/H
				if bid.W == 0 && bid.H == 0 {
					bid.W, bid.H = imp.Width, imp.Height
				}

				if imp.IsDirect() {
					// Custom direct detect
					if bid.AdMarkup == "" {
						bid.AdMarkup, _ = customDirectURL(bid.Ext)
					}
					if strings.HasPrefix(bid.AdMarkup, `<?xml`) {
						bid.AdMarkup, _ = decodePopMarkup([]byte(bid.AdMarkup))
					}
				}
			}

			replacer := r.newBidReplacer(&bid)
			bid.AdMarkup = replacer.Replace(bid.AdMarkup)
			bid.NURL = prepareURL(bid.NURL, replacer)
			bid.BURL = prepareURL(bid.BURL, replacer)

			seat.Bid[i] = bid
		}

		r.BidResponse.SeatBid[i] = seat
		r.bidRespBidCount += len(seat.Bid)
	} // end for

	for _, bid := range r.OptimalBids() {
		imp := r.Req.ImpressionByIDvariation(bid.ImpID)
		if imp == nil {
			continue
		}

		if imp.IsDirect() {
			format := imp.FormatByType(types.FormatDirectType)
			if format == nil {
				continue
			}
			r.ads = append(r.ads, &ResponseBidItem{
				ItemID:     imp.ID,
				Src:        r.Src,
				Req:        r.Req,
				Imp:        imp,
				FormatType: types.FormatDirectType,
				RespFormat: format,
				Bid:        bid,
				ActionLink: bid.AdMarkup,
				PriceScope: adtype.PriceScopeView{
					TestViewBudget: false,
					MaxBidPrice:    billing.MoneyFloat(bid.Price),
					BidPrice:       billing.MoneyFloat(bid.Price),
					ViewPrice:      billing.MoneyFloat(bid.Price),
					ECPM:           billing.MoneyFloat(bid.Price),
				},
			})
			continue
		}

		for _, format := range imp.Formats() {
			if bid.ImpID != imp.IDByFormat(format) {
				continue
			}
			switch {
			case format.IsNative():
				native, err := decodeNativeMarkup([]byte(bid.AdMarkup))
				if err == nil {
					bidItem := &ResponseBidItem{
						ItemID:     imp.ID,
						Src:        r.Src,
						Req:        r.Req,
						Imp:        imp,
						FormatType: types.FormatNativeType,
						RespFormat: format,
						Bid:        bid,
						Native:     native,
						ActionLink: native.Link.URL,
						PriceScope: adtype.PriceScopeView{
							TestViewBudget: false,
							MaxBidPrice:    billing.MoneyFloat(bid.Price),
							BidPrice:       billing.MoneyFloat(bid.Price),
							ViewPrice:      billing.MoneyFloat(bid.Price),
							ECPM:           billing.MoneyFloat(bid.Price),
						},
					}
					if nativeRequestV2 := imp.RTBNativeRequest(); nativeRequestV2 != nil {
						bidItem.Data = extractNativeV2Data(nativeRequestV2, native)
					} else if nativeRequestV3 := imp.RTBNativeRequestV3(); nativeRequestV3 != nil {
						bidItem.Data = extractNativeV3Data(nativeRequestV3, native)
					}
					r.ads = append(r.ads, bidItem)
				} else {
					ctxlogger.Get(r.Context()).Debug(
						"Failed to decode native markup",
						zap.String("markup", bid.AdMarkup),
						zap.Error(err),
					)
				}
			case format.IsBanner() || format.IsProxy():
				r.ads = append(r.ads, &ResponseBidItem{
					ItemID:     imp.ID,
					Src:        r.Src,
					Req:        r.Req,
					Imp:        imp,
					FormatType: bannerFormatType(bid.AdMarkup),
					RespFormat: format,
					Bid:        bid,
					PriceScope: adtype.PriceScopeView{
						TestViewBudget: false,
						MaxBidPrice:    billing.MoneyFloat(bid.Price),
						BidPrice:       billing.MoneyFloat(bid.Price),
						ViewPrice:      billing.MoneyFloat(bid.Price),
						ECPM:           billing.MoneyFloat(bid.Price),
					},
				})
			}
			break
		}
	}
}

// Request information
func (r *BidResponse) Request() *adtype.BidRequest {
	return r.Req
}

// Ads list
func (r *BidResponse) Ads() []adtype.ResponserItemCommon {
	return r.ads
}

// Item by impression code
func (r *BidResponse) Item(impid string) adtype.ResponserItemCommon {
	for _, it := range r.Ads() {
		if it.ImpressionID() == impid {
			return it
		}
	}
	return nil
}

// Count bids
func (r *BidResponse) Count() int {
	return r.bidRespBidCount
}

// Validate response
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

// Error of the response
func (r *BidResponse) Error() error {
	return r.Validate()
}

// OptimalBids list (the most expensive)
func (r *BidResponse) OptimalBids() []*openrtb.Bid {
	if len(r.optimalBids) > 0 {
		return r.optimalBids
	}

	bids := make(map[string]*openrtb.Bid, len(r.BidResponse.SeatBid))
	for _, seat := range r.BidResponse.SeatBid {
		for _, bid := range seat.Bid {
			if obid, ok := bids[bid.ImpID]; !ok || obid.Price < bid.Price {
				bids[bid.ImpID] = &bid
			}
		}
	}

	optimalBids := make([]*openrtb.Bid, 0, len(bids))
	for _, b := range bids {
		optimalBids = append(optimalBids, b)
	}
	r.optimalBids = optimalBids
	return r.optimalBids
}

// Context of response
func (r *BidResponse) Context(ctx ...context.Context) context.Context {
	if len(ctx) > 0 {
		r.context = ctx[0]
	}
	if r.context == nil {
		return r.Req.Ctx
	}
	return r.context
}

// Get context value
func (r *BidResponse) Get(key string) any {
	if r.context != nil {
		return r.context.Value(key)
	}
	return nil
}

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

// Release response and all linked objects
func (r *BidResponse) Release() {
	if r == nil {
		return
	}
	if r.Req != nil {
		r.Req.Release()
		r.Req = nil
	}
	r.ads = r.ads[:0]
	r.optimalBids = r.optimalBids[:0]
	r.BidResponse.SeatBid = r.BidResponse.SeatBid[:0]
	r.BidResponse.Ext = r.BidResponse.Ext[:0]
}

var (
	_ adtype.Responser = &BidResponse{}
)
