//
// @project GeniusRabbit corelib 2017 - 2019, 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019, 2025
//

package adresponse

import (
	"context"

	"github.com/demdxx/gocast/v2"

	"github.com/bsm/openrtb"
	natresp "github.com/bsm/openrtb/native/response"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/billing"
	"github.com/geniusrabbit/adcorelib/price"
)

// ResponseNativeBidItem is the response item for native bid format
type ResponseNativeBidItem struct {
	ItemID string `json:"id"`

	// Request and impression data
	Src adtype.Source       `json:"source,omitempty"`
	Req adtype.BidRequester `json:"request,omitempty"`
	Imp *adtype.Impression  `json:"impression,omitempty"`

	// Format of response advertisement item
	FormatType types.FormatType `json:"format_type,omitempty"`
	RespFormat *types.Format    `json:"format,omitempty"`

	// External response data from RTB source
	Bid        *openrtb.Bid      `json:"bid,omitempty"`
	Native     *natresp.Response `json:"native,omitempty"`
	ActionLink string            `json:"action_link,omitempty"`

	PriceScope price.PriceScopeImpression `json:"price_scope,omitempty"`

	// Competitive second AD
	SecondAd adtype.SecondAd `json:"second_ad,omitempty"`

	Data    map[string]any        `json:"data,omitempty"`
	assets  admodels.AdFileAssets `json:"-"`
	context context.Context       `json:"-"`
}

func newResponseNativeBidItem(req adtype.BidRequester, src adtype.Source, bid *openrtb.Bid, imp *adtype.Impression, format *types.Format) (*ResponseNativeBidItem, error) {
	// Handle native ad format with structured data
	native, err := decodeNativeMarkup([]byte(bid.AdMarkup))
	if err != nil {
		return nil, err
	}

	// Calculate the bid price and set up the price scope for the bid item
	cpmPrice := billing.MoneyFloat(bid.Price)

	// Set the bid item properties
	priceScope := price.PriceScopeImpression{
		MaxBidImpPrice: 0,
		BidImpPrice:    0,
		ImpPrice:       cpmPrice / 1000, // Convert from micros (CPM) to actual price
		ECPM:           cpmPrice,        // Original eCPM price
	}

	// Create bid item for native format
	bidItem := &ResponseNativeBidItem{
		ItemID:     imp.ID,
		Src:        src,
		Req:        req,
		Imp:        imp,
		Bid:        bid,
		FormatType: types.FormatNativeType,
		RespFormat: format,
		Native:     native,
		ActionLink: native.Link.URL,
		Data:       extractNativeDataFromImpression(imp, native),
		PriceScope: priceScope,
	}

	// Set the bid impression price based on the bid price and impression
	bidItem.PriceScope.MaxBidImpPrice = price.CalculatePurchasePrice(bidItem, adtype.ActionImpression)

	return bidItem, nil
}

// ID of current response item (unique code of current response)
func (it *ResponseNativeBidItem) ID() string {
	return it.ItemID
}

// Source of response
func (it *ResponseNativeBidItem) Source() adtype.Source {
	return it.Src
}

// NetworkName by source
func (it *ResponseNativeBidItem) NetworkName() string {
	return ""
}

// ContentItemString from the ad
func (it *ResponseNativeBidItem) ContentItemString(name string) string {
	if val := it.ContentItem(name); val != nil {
		return gocast.Str(val)
	}
	return ""
}

// ContentItem returns the ad response data
func (it *ResponseNativeBidItem) ContentItem(name string) any {
	if it.Data != nil {
		return it.Data[name]
	}

	switch name {
	case adtype.ContentItemLink:
		return it.Native.Link.URL
	case adtype.ContentItemNotifyWinURL:
		if it.Bid != nil {
			return it.Bid.NURL
		}
	case adtype.ContentItemNotifyDisplayURL:
		if it.Bid != nil {
			return it.Bid.BURL
		}
	case types.FormatFieldTitle:
		for _, asset := range it.Native.Assets {
			if asset.Title != nil {
				return asset.Title.Text
			}
		}
	default:
		for _, asset := range it.Native.Assets {
			if asset.Data != nil && asset.Data.Label == name {
				return asset.Data.Value
			}
		}
	}
	return nil
}

// ContentFields from advertisement object
func (it *ResponseNativeBidItem) ContentFields() map[string]any {
	if it.Format().Config == nil {
		return nil
	}
	fields := map[string]any{}
	config := it.Format().Config
	for _, field := range config.Fields {
		for _, asset := range it.Native.Assets {
			if field.ID != asset.ID {
				continue
			}
			switch {
			case asset.Title != nil:
				fields[field.Name] = asset.Title.Text
			case asset.Link != nil:
				fields[field.Name] = asset.Link.URL
			case asset.Data != nil:
				fields[field.Name] = asset.Data.Value
			}
			break
		}
	}
	return fields
}

// ImpressionTrackerLinks returns traking links for impression action
func (it *ResponseNativeBidItem) ImpressionTrackerLinks() []string {
	return it.Native.ImpTrackers
}

// ViewTrackerLinks returns traking links for view action
func (it *ResponseNativeBidItem) ViewTrackerLinks() []string {
	return nil
}

// ClickTrackerLinks returns third-party tracker URLs to be fired on click of the URL
func (it *ResponseNativeBidItem) ClickTrackerLinks() []string {
	return it.Native.Link.ClickTrackers
}

// MainAsset from response
func (it *ResponseNativeBidItem) MainAsset() *admodels.AdFileAsset {
	mainAsset := it.Format().Config.MainAsset()
	if mainAsset == nil {
		return nil
	}
	for _, asset := range it.Assets() {
		if int(asset.ID) == mainAsset.ID {
			return asset
		}
	}
	return nil
}

// Assets returns list of the advertisement
func (it *ResponseNativeBidItem) Assets() (assets admodels.AdFileAssets) {
	if it.assets != nil || it.Format().Config == nil {
		return it.assets
	}

	config := it.Format().Config
	for _, configAsset := range config.Assets {
		for _, asset := range it.Native.Assets {
			if asset.ID != configAsset.ID && (asset.Image != nil || asset.Video != nil) {
				continue
			}
			newAsset := &admodels.AdFileAsset{
				ID:   uint64(asset.ID),
				Name: configAsset.GetName(),
			}
			switch {
			case asset.Image != nil:
				newAsset.URL = asset.Image.URL
				newAsset.Type = types.AdFileAssetImageType
				newAsset.ContentType = ""
				newAsset.Width = asset.Image.Width
				newAsset.Height = asset.Image.Height
			case asset.Video != nil:
				newAsset.URL = asset.Video.VASTTag
				newAsset.Type = types.AdFileAssetVideoType
			}
			it.assets = append(it.assets, newAsset)
			break
		}
	}
	return it.assets
}

// Format object model
func (it *ResponseNativeBidItem) Format() *types.Format {
	if it == nil {
		return nil
	}
	return it.RespFormat
}

// PriorityFormatType from current Ad
func (it *ResponseNativeBidItem) PriorityFormatType() types.FormatType {
	if it.FormatType != types.FormatUndefinedType {
		return it.FormatType
	}
	format := it.Imp.FormatTypes
	if formatType := format.HasOneType(); formatType > types.FormatUndefinedType {
		return formatType
	}
	return format.FirstType()
}

// Impression place object
func (it *ResponseNativeBidItem) Impression() *adtype.Impression {
	return it.Imp
}

// ImpressionID unique code string
func (it *ResponseNativeBidItem) ImpressionID() string {
	if it.Imp == nil {
		return ""
	}
	return it.Imp.ID
}

// ExtImpressionID unique code of RTB response
func (it *ResponseNativeBidItem) ExtImpressionID() string {
	if it.Imp == nil {
		return ""
	}
	return it.Imp.ExternalID
}

// ExtTargetID of the external network
func (it *ResponseNativeBidItem) ExtTargetID() string {
	return it.Imp.ExternalTargetID
}

// TargetCodename of the target placement codename
func (it *ResponseNativeBidItem) TargetCodename() string {
	return it.Imp.TargetCodename()
}

// AdID returns the advertisement ID of the system
func (it *ResponseNativeBidItem) AdID() string {
	return ""
}

// CreativeID of the external advertisement
func (it *ResponseNativeBidItem) CreativeID() string {
	if it == nil || it.Bid == nil {
		return ""
	}
	return it.Bid.CreativeID
}

// AccountID returns the account ID of the source
func (it *ResponseNativeBidItem) AccountID() uint64 {
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
func (it *ResponseNativeBidItem) CampaignID() uint64 {
	return 0
}

///////////////////////////////////////////////////////////////////////////////
// Price calculation methods
///////////////////////////////////////////////////////////////////////////////

// PricingModel of advertisement
// In case of RTB it can be CPM only
func (it *ResponseNativeBidItem) PricingModel() types.PricingModel {
	return types.PricingModelCPM
}

// FixedPurchasePrice returns the fixed price of the action
func (it *ResponseNativeBidItem) FixedPurchasePrice(action adtype.Action) billing.Money {
	return it.Imp.PurchasePrice(action)
}

// ECPM returns the effective cost per mille
func (it *ResponseNativeBidItem) ECPM() billing.Money {
	if it == nil || it.Bid == nil {
		return 0
	}
	return it.PriceScope.ECPM
}

// PriceTestMode returns true if the price is in test mode
func (it *ResponseNativeBidItem) PriceTestMode() bool { return false }

// Price for specific action if supported `click`, `lead`, `view`
// returns total price of the action
func (it *ResponseNativeBidItem) Price(action adtype.Action) billing.Money {
	if it == nil || it.Bid == nil {
		return 0
	}
	price := it.PriceScope.PricePerAction(action)
	return price
}

// BidViewPrice returns bid price for the external auction source.
// The current bid price will be adjusted according to the source correction factor and the commission share factor
func (it *ResponseNativeBidItem) BidImpressionPrice() billing.Money {
	return it.PriceScope.BidImpPrice
}

// SetBidImpressionPrice value for external sources auction the system will pay
func (it *ResponseNativeBidItem) SetBidImpressionPrice(bid billing.Money) error {
	if !it.PriceScope.SetBidImpressionPrice(bid, false) {
		return adtype.ErrNewAuctionBidIsHigherThenMaxBid
	}
	return nil
}

// PrepareBidImpressionPrice prepares the price for the action
// The price is adjusted according to the source correction factor and the commission share factor
func (it *ResponseNativeBidItem) PrepareBidImpressionPrice(price billing.Money) billing.Money {
	return it.PriceScope.PrepareBidImpressionPrice(price)
}

// InternalAuctionCPMBid value provides maximal possible price without any commission
// According to this value the system can choice the best item for the auction
func (it *ResponseNativeBidItem) InternalAuctionCPMBid() billing.Money {
	return price.CalculateInternalAuctionBid(it)
}

// PurchasePrice gives the price of view from external resource.
// The cost of this request for the system.
func (it *ResponseNativeBidItem) PurchasePrice(action adtype.Action) billing.Money {
	return price.CalculatePurchasePrice(it, action)
}

// PotentialPrice wich can be received from source but was marked as descrepancy
func (it *ResponseNativeBidItem) PotentialPrice(action adtype.Action) billing.Money {
	return price.CalculatePotentialPrice(it, action)
}

// FinalPrice for the action with all corrections and commissions
func (it *ResponseNativeBidItem) FinalPrice(action adtype.Action) billing.Money {
	return price.CalculateFinalPrice(it, action)
}

// Second campaigns
func (it *ResponseNativeBidItem) Second() *adtype.SecondAd {
	return &it.SecondAd
}

///////////////////////////////////////////////////////////////////////////////
// Revenue share/comission methods
///////////////////////////////////////////////////////////////////////////////

// CommissionShareFactor which system get from publisher 0..1
func (it *ResponseNativeBidItem) CommissionShareFactor() float64 {
	return it.Imp.CommissionShareFactor()
}

// SourceCorrectionFactor value for the source
func (it *ResponseNativeBidItem) SourceCorrectionFactor() float64 {
	return it.Src.PriceCorrectionReduceFactor()
}

// TargetCorrectionFactor value for the target
func (it *ResponseNativeBidItem) TargetCorrectionFactor() float64 {
	return it.Imp.Target.RevenueShareReduceFactor()
}

///////////////////////////////////////////////////////////////////////////////
// Other methods
///////////////////////////////////////////////////////////////////////////////

// RTBCategories of the advertisement
func (it *ResponseNativeBidItem) RTBCategories() []string {
	if it.Bid == nil {
		return nil
	}
	return it.Bid.Cat
}

// IsDirect AD format
func (it *ResponseNativeBidItem) IsDirect() bool {
	return it.Imp.IsDirect()
}

// IsBackup indicates whether the advertisement is a backup ad type.
func (it *ResponseNativeBidItem) IsBackup() bool { return false }

// ActionURL for direct ADS
func (it *ResponseNativeBidItem) ActionURL() string {
	return it.ActionLink
}

// Validate item
func (it *ResponseNativeBidItem) Validate() error {
	if it.Src == nil || it.Req == nil || it.Imp == nil || it.Bid == nil {
		return adtype.ErrInvalidItemInitialisation
	}
	return it.Bid.Validate()
}

// Width of item
func (it *ResponseNativeBidItem) Width() int {
	if it.Bid == nil {
		return 0
	}
	return it.Bid.W
}

// Height of item
func (it *ResponseNativeBidItem) Height() int {
	if it.Bid == nil {
		return 0
	}
	return it.Bid.H
}

// Markup advertisement
func (it *ResponseNativeBidItem) Markup() (string, error) {
	return "", nil
}

///////////////////////////////////////////////////////////////////////////////
// Context methods
///////////////////////////////////////////////////////////////////////////////

// Context value
func (it *ResponseNativeBidItem) Context(ctx ...context.Context) context.Context {
	if len(ctx) > 0 {
		it.context = ctx[0]
	}
	return it.context
}

// Get ext field
func (it *ResponseNativeBidItem) Get(key string) (res any) {
	if it.context == nil {
		return res
	}
	return it.context.Value(key)
}

var (
	_ adtype.ResponseItem = &ResponseNativeBidItem{}
)
