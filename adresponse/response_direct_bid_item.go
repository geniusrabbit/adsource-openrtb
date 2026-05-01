//
// @project GeniusRabbit corelib 2017 - 2019, 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019, 2025
//

package adresponse

import (
	"context"
	"strings"

	"github.com/demdxx/gocast/v2"

	"github.com/bsm/openrtb"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/billing"
	"github.com/geniusrabbit/adcorelib/price"
)

// ResponseDirectBidItem is the response item for direct bid format
type ResponseDirectBidItem struct {
	ItemID string `json:"id"`

	// Request and impression data
	Src adtype.Source       `json:"source,omitempty"`
	Req adtype.BidRequester `json:"request,omitempty"`
	Imp *adtype.Impression  `json:"impression,omitempty"`

	// Format of response advertisement item
	FormatType types.FormatType `json:"format_type,omitempty"`
	RespFormat *types.Format    `json:"format,omitempty"`

	// External response data from RTB source
	Bid        *openrtb.Bid `json:"bid,omitempty"`
	DirectLink string       `json:"action_link,omitempty"`

	PriceScope price.PriceScopeImpression `json:"price_scope,omitempty"`

	// Competitive second AD
	SecondAd adtype.SecondAd `json:"second_ad,omitempty"`

	assets  admodels.AdFileAssets `json:"-"`
	context context.Context       `json:"-"`
}

func newResponseDirectBidItem(req adtype.BidRequester, src adtype.Source, bid *openrtb.Bid, imp *adtype.Impression, format *types.Format) (*ResponseDirectBidItem, error) {
	// Calculate the bid price and set up the price scope for the bid item
	cpmPrice := billing.MoneyFloat(bid.Price)

	// Set the bid item properties
	priceScope := price.PriceScopeImpression{
		MaxBidImpPrice: 0,
		BidImpPrice:    0,
		ImpPrice:       cpmPrice / 1000, // Convert from micros (CPM) to actual price
		ECPM:           cpmPrice,        // Original eCPM price
	}

	// Handle direct response format (like click URLs)
	bidItem := &ResponseDirectBidItem{
		ItemID:     imp.ID,
		Src:        src,
		Req:        req,
		Imp:        imp,
		Bid:        bid,
		FormatType: types.FormatDirectType,
		RespFormat: format,
		PriceScope: priceScope,
	}

	// Determine the content of the direct ad based on the ad markup
	if strings.HasPrefix(bid.AdMarkup, "https://") ||
		strings.HasPrefix(bid.AdMarkup, "http://") ||
		strings.HasPrefix(bid.AdMarkup, "//") {
		bidItem.DirectLink = bid.AdMarkup
	} else if strings.HasPrefix(bid.AdMarkup, "<?xml") {
		if popURL, _, imgURL, err := parseInterstitialAdMarkup(bid.AdMarkup); err != nil || imgURL != "" {
			if imgURL != "" {
				return nil, ErrInvalidAdContent
			}
			return nil, err
		} else {
			bidItem.DirectLink = popURL
		}
	}

	// Set the bid impression price based on the bid price and impression
	bidItem.PriceScope.MaxBidImpPrice = price.CalculatePurchasePrice(bidItem, adtype.ActionImpression)

	return bidItem, nil
}

// ID of current response item (unique code of current response)
func (it *ResponseDirectBidItem) ID() string {
	return it.ItemID
}

// Source of response
func (it *ResponseDirectBidItem) Source() adtype.Source {
	return it.Src
}

// NetworkName by source
func (it *ResponseDirectBidItem) NetworkName() string {
	return ""
}

// ContentItemString from the ad
func (it *ResponseDirectBidItem) ContentItemString(name string) string {
	if val := it.ContentItem(name); val != nil {
		return gocast.Str(val)
	}
	return ""
}

// ContentItem returns the ad response data
func (it *ResponseDirectBidItem) ContentItem(name string) any {
	switch name {
	case adtype.ContentItemContent, adtype.ContentItemIFrameURL:
		switch name {
		case adtype.ContentItemIFrameURL:
			return it.DirectLink
		case adtype.ContentItemContent:
			return `<iframe src="` + it.DirectLink + `" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; fullscreen" referrerpolicy="no-referrer" sandbox="allow-scripts allow-same-origin allow-popups allow-top-navigation-by-user-activation" frameborder="0" style="width:100%;height:100%;"></iframe>`
		}
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

// ContentFields from advertisement object
func (it *ResponseDirectBidItem) ContentFields() map[string]any {
	return nil
}

// ImpressionTrackerLinks returns traking links for impression action
func (it *ResponseDirectBidItem) ImpressionTrackerLinks() []string {
	return nil
}

// ViewTrackerLinks returns traking links for view action
func (it *ResponseDirectBidItem) ViewTrackerLinks() []string {
	return nil
}

// ClickTrackerLinks returns third-party tracker URLs to be fired on click of the URL
func (it *ResponseDirectBidItem) ClickTrackerLinks() []string {
	return nil
}

// MainAsset from response
func (it *ResponseDirectBidItem) MainAsset() *admodels.AdFileAsset {
	return nil
}

// Assets returns list of the advertisement
func (it *ResponseDirectBidItem) Assets() admodels.AdFileAssets {
	return nil
}

// Format object model
func (it *ResponseDirectBidItem) Format() *types.Format {
	if it == nil {
		return nil
	}
	return it.RespFormat
}

// PriorityFormatType from current Ad
func (it *ResponseDirectBidItem) PriorityFormatType() types.FormatType {
	return types.FormatDirectType
}

// Impression place object
func (it *ResponseDirectBidItem) Impression() *adtype.Impression {
	return it.Imp
}

// ImpressionID unique code string
func (it *ResponseDirectBidItem) ImpressionID() string {
	if it.Imp == nil {
		return ""
	}
	return it.Imp.ID
}

// ExtImpressionID unique code of RTB response
func (it *ResponseDirectBidItem) ExtImpressionID() string {
	if it.Imp == nil {
		return ""
	}
	return it.Imp.ExternalID
}

// ExtTargetID of the external network
func (it *ResponseDirectBidItem) ExtTargetID() string {
	return it.Imp.ExternalTargetID
}

// TargetCodename of the target placement codename
func (it *ResponseDirectBidItem) TargetCodename() string {
	return it.Imp.TargetCodename()
}

// AdID returns the advertisement ID of the system
func (it *ResponseDirectBidItem) AdID() string {
	return ""
}

// CreativeID of the external advertisement
func (it *ResponseDirectBidItem) CreativeID() string {
	if it == nil || it.Bid == nil {
		return ""
	}
	return it.Bid.CreativeID
}

// AccountID returns the account ID of the source
func (it *ResponseDirectBidItem) AccountID() uint64 {
	if it.Src != nil {
		type accountIDGetter interface {
			AccountID() uint64
		}
		if src, _ := it.Src.(accountIDGetter); src != nil {
			return src.AccountID()
		}
	}
	return 0
}

// CampaignID returns the campaign ID of the system
func (it *ResponseDirectBidItem) CampaignID() uint64 {
	return 0
}

///////////////////////////////////////////////////////////////////////////////
// Price calculation methods
///////////////////////////////////////////////////////////////////////////////

// PricingModel of advertisement
// In case of RTB it can be CPM only
func (it *ResponseDirectBidItem) PricingModel() types.PricingModel {
	return types.PricingModelCPM
}

// FixedPurchasePrice returns the fixed price of the action
func (it *ResponseDirectBidItem) FixedPurchasePrice(action adtype.Action) billing.Money {
	return it.Imp.PurchasePrice(action)
}

// ECPM returns the effective cost per mille
func (it *ResponseDirectBidItem) ECPM() billing.Money {
	if it == nil || it.Bid == nil {
		return 0
	}
	return it.PriceScope.ECPM
}

// PriceTestMode returns true if the price is in test mode
func (it *ResponseDirectBidItem) PriceTestMode() bool { return false }

// Price for specific action if supported `click`, `lead`, `view`
// returns total price of the action
func (it *ResponseDirectBidItem) Price(action adtype.Action) billing.Money {
	if it == nil || it.Bid == nil {
		return 0
	}
	price := it.PriceScope.PricePerAction(action)
	return price
}

// BidViewPrice returns bid price for the external auction source.
// The current bid price will be adjusted according to the source correction factor and the commission share factor
func (it *ResponseDirectBidItem) BidImpressionPrice() billing.Money {
	return it.PriceScope.BidImpPrice
}

// SetBidImpressionPrice value for external sources auction the system will pay
func (it *ResponseDirectBidItem) SetBidImpressionPrice(bid billing.Money) error {
	if !it.PriceScope.SetBidImpressionPrice(bid, false) {
		return adtype.ErrNewAuctionBidIsHigherThenMaxBid
	}
	return nil
}

// PrepareBidImpressionPrice prepares the price for the action
// The price is adjusted according to the source correction factor and the commission share factor
func (it *ResponseDirectBidItem) PrepareBidImpressionPrice(price billing.Money) billing.Money {
	return it.PriceScope.PrepareBidImpressionPrice(price)
}

// InternalAuctionCPMBid value provides maximal possible price without any commission
// According to this value the system can choice the best item for the auction
func (it *ResponseDirectBidItem) InternalAuctionCPMBid() billing.Money {
	return price.CalculateInternalAuctionBid(it)
}

// PurchasePrice gives the price of view from external resource.
// The cost of this request for the system.
func (it *ResponseDirectBidItem) PurchasePrice(action adtype.Action) billing.Money {
	return price.CalculatePurchasePrice(it, action)
}

// PotentialPrice wich can be received from source but was marked as descrepancy
func (it *ResponseDirectBidItem) PotentialPrice(action adtype.Action) billing.Money {
	return price.CalculatePotentialPrice(it, action)
}

// FinalPrice for the action with all corrections and commissions
func (it *ResponseDirectBidItem) FinalPrice(action adtype.Action) billing.Money {
	return price.CalculateFinalPrice(it, action)
}

// Second campaigns
func (it *ResponseDirectBidItem) Second() *adtype.SecondAd {
	return &it.SecondAd
}

///////////////////////////////////////////////////////////////////////////////
// Revenue share/comission methods
///////////////////////////////////////////////////////////////////////////////

// CommissionShareFactor which system get from publisher 0..1
func (it *ResponseDirectBidItem) CommissionShareFactor() float64 {
	return it.Imp.CommissionShareFactor()
}

// SourceCorrectionFactor value for the source
func (it *ResponseDirectBidItem) SourceCorrectionFactor() float64 {
	return it.Src.PriceCorrectionReduceFactor()
}

// TargetCorrectionFactor value for the target
func (it *ResponseDirectBidItem) TargetCorrectionFactor() float64 {
	return it.Imp.Target.RevenueShareReduceFactor()
}

///////////////////////////////////////////////////////////////////////////////
// Other methods
///////////////////////////////////////////////////////////////////////////////

// RTBCategories of the advertisement
func (it *ResponseDirectBidItem) RTBCategories() []string {
	if it.Bid == nil {
		return nil
	}
	return it.Bid.Cat
}

// IsDirect AD format
func (it *ResponseDirectBidItem) IsDirect() bool {
	return true
}

// IsBackup indicates whether the advertisement is a backup ad type.
func (it *ResponseDirectBidItem) IsBackup() bool { return false }

// ActionURL for direct ADS
func (it *ResponseDirectBidItem) ActionURL() string {
	return it.DirectLink
}

// Validate item
func (it *ResponseDirectBidItem) Validate() error {
	if it.Src == nil || it.Req == nil || it.Imp == nil || it.Bid == nil {
		return adtype.ErrInvalidItemInitialisation
	}
	return it.Bid.Validate()
}

// Width of item
func (it *ResponseDirectBidItem) Width() int {
	if it.Bid == nil {
		return 0
	}
	return it.Bid.W
}

// Height of item
func (it *ResponseDirectBidItem) Height() int {
	if it.Bid == nil {
		return 0
	}
	return it.Bid.H
}

// Markup advertisement
func (it *ResponseDirectBidItem) Markup() (string, error) {
	return "", nil
}

///////////////////////////////////////////////////////////////////////////////
// Context methods
///////////////////////////////////////////////////////////////////////////////

// Context value
func (it *ResponseDirectBidItem) Context(ctx ...context.Context) context.Context {
	if len(ctx) > 0 {
		it.context = ctx[0]
	}
	return it.context
}

// Get ext field
func (it *ResponseDirectBidItem) Get(key string) (res any) {
	if it.context == nil {
		return res
	}
	return it.context.Value(key)
}

var (
	_ adtype.ResponseItem = &ResponseDirectBidItem{}
)
