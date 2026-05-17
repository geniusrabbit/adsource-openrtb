//
// @project GeniusRabbit corelib 2017 - 2019, 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019, 2025
//

// Package direct implements the OpenRTB bid response item for direct (click URL) ad formats.
// Direct ads deliver a click-through URL rather than full creative markup; the system
// renders an iframe or redirect wrapper around that URL.
package direct

import (
	"errors"
	"strings"

	"github.com/bsm/openrtb"
	"github.com/demdxx/gocast/v2"

	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/billing"
	"github.com/geniusrabbit/adcorelib/price"

	"github.com/geniusrabbit/adsource-openrtb/response/common"
	"github.com/geniusrabbit/adsource-openrtb/response/internal/interstitial"
)

// ErrInvalidAdContent is returned when the ad markup cannot be resolved to a direct link.
var ErrInvalidAdContent = errors.New("invalid ad content")

// ResponseBidItem is the bid response item for direct ad formats.
// It implements [adtype.ResponseItem] and contains the resolved click-through URL
// together with pricing and tracking information.
type ResponseBidItem struct {
	common.BaseBidItem
	DirectLink string `json:"action_link,omitempty"`
}

// New creates a ResponseBidItem for a direct bid. It resolves the ad markup to a
// click-through URL, returning [ErrInvalidAdContent] when only an image URL is found
// (which is not a valid direct link).
func New(req adtype.BidRequester, src adtype.Source, bid *openrtb.Bid, imp *adtype.Impression, format *types.Format) (*ResponseBidItem, error) {
	cpmPrice := billing.MoneyFloat(bid.Price)
	bidItem := &ResponseBidItem{
		BaseBidItem: common.BaseBidItem{
			ItemID:     imp.ID,
			Src:        src,
			Req:        req,
			Imp:        imp,
			Bid:        bid,
			FormatType: types.FormatDirectType,
			RespFormat: format,
			PriceScope: price.PriceScopeImpression{
				MaxBidImpPrice: 0,
				BidImpPrice:    0,
				ImpPrice:       cpmPrice / 1000, // Convert CPM to per-impression price
				ECPM:           cpmPrice,
			},
		},
	}

	switch {
	case strings.HasPrefix(bid.AdMarkup, "https://") ||
		strings.HasPrefix(bid.AdMarkup, "http://") ||
		strings.HasPrefix(bid.AdMarkup, "//"):
		bidItem.DirectLink = bid.AdMarkup
	case strings.HasPrefix(bid.AdMarkup, "<?xml"):
		popURL, _, imgURL, err := interstitial.ParseAdMarkup(bid.AdMarkup)
		if err != nil {
			return nil, err
		}
		if imgURL != "" {
			return nil, ErrInvalidAdContent
		}
		bidItem.DirectLink = popURL
	}

	// Ensure a valid direct link was resolved from the ad markup.
	bidItem.PriceScope.MaxBidImpPrice =
		price.CalculatePurchasePrice(bidItem, adtype.ActionImpression)

	return bidItem, nil
}

// ContentItemString returns the string value of a named content field.
func (it *ResponseBidItem) ContentItemString(name string) string {
	if val := it.ContentItem(name); val != nil {
		return gocast.Str(val)
	}
	return ""
}

// ContentItem returns the ad response data for the given field name.
func (it *ResponseBidItem) ContentItem(name string) any {
	switch name {
	case adtype.ContentItemIFrameURL:
		return it.DirectLink
	case adtype.ContentItemContent:
		return `<iframe src="` + it.DirectLink + `" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; fullscreen" referrerpolicy="no-referrer" sandbox="allow-scripts allow-same-origin allow-popups allow-top-navigation-by-user-activation" frameborder="0" style="width:100%;height:100%;"></iframe>`
	case adtype.ContentItemLink:
		return it.DirectLink
	case adtype.ContentItemNotifyWinURL:
		if it.Bid != nil {
			return it.Bid.NURL
		}
	case adtype.ContentItemNotifyDisplayURL:
		if it.Bid != nil {
			return it.Bid.BURL
		}
	}
	return nil
}

// ContentFields returns a map of all populated content fields (nil for direct format).
func (it *ResponseBidItem) ContentFields() map[string]any {
	return nil
}

// ActionURL returns the resolved direct click-through URL.
func (it *ResponseBidItem) ActionURL() string {
	return it.DirectLink
}

// IsDirect always returns true for direct bid items.
func (it *ResponseBidItem) IsDirect() bool {
	return true
}

// PriorityFormatType always returns FormatDirectType for direct bid items.
func (it *ResponseBidItem) PriorityFormatType() types.FormatType {
	return types.FormatDirectType
}

var _ adtype.ResponseItem = &ResponseBidItem{}
