//
// @project GeniusRabbit corelib 2017 - 2019
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019
//

package adresponse

import (
	"context"
	"strings"

	"github.com/demdxx/gocast/v2"

	"github.com/bsm/openrtb"
	natresp "github.com/bsm/openrtb/native/response"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/billing"
	"github.com/geniusrabbit/adcorelib/price"
)

// ResponseBidItem value
type ResponseBidItem struct {
	ItemID string `json:"id"`

	// Request and impression data
	Src adtype.Source      `json:"source,omitempty"`
	Req *adtype.BidRequest `json:"request,omitempty"`
	Imp *adtype.Impression `json:"impression,omitempty"`

	// Format of response advertisement item
	FormatType types.FormatType `json:"format_type,omitempty"`
	RespFormat *types.Format    `json:"format,omitempty"`

	// External response data from RTB source
	Bid        *openrtb.Bid      `json:"bid,omitempty"`
	Native     *natresp.Response `json:"native,omitempty"`
	ActionLink string            `json:"action_link,omitempty"`

	PriceScope price.PriceScopeView `json:"price_scope,omitempty"`

	// Competitive second AD
	SecondAd adtype.SecondAd `json:"second_ad,omitempty"`

	Data    map[string]any    `json:"data,omitempty"`
	assets  admodels.AdAssets `json:"-"`
	context context.Context   `json:"-"`
}

// ID of current response item (unique code of current response)
func (it *ResponseBidItem) ID() string {
	return it.ItemID
}

// Source of response
func (it *ResponseBidItem) Source() adtype.Source {
	return it.Src
}

// NetworkName by source
func (it *ResponseBidItem) NetworkName() string {
	return ""
}

// ContentItemString from the ad
func (it *ResponseBidItem) ContentItemString(name string) string {
	val := it.ContentItem(name)
	if val != nil {
		return gocast.Str(val)
	}
	return ""
}

// ContentItem returns the ad response data
func (it *ResponseBidItem) ContentItem(name string) any {
	if it.Data != nil {
		return it.Data[name]
	}

	formatType := it.PriorityFormatType()

	switch name {
	case adtype.ContentItemContent, adtype.ContentItemIFrameURL:
		if formatType.IsBanner() {
			switch name {
			case adtype.ContentItemIFrameURL:
				if strings.HasPrefix(it.Bid.AdMarkup, "http://") ||
					strings.HasPrefix(it.Bid.AdMarkup, "https://") ||
					(strings.HasPrefix(it.Bid.AdMarkup, "//") && !strings.ContainsAny(it.Bid.AdMarkup, "\n\t")) {
					return it.Bid.AdMarkup
				}
			case adtype.ContentItemContent:
				return it.Bid.AdMarkup
			}
		}
	case adtype.ContentItemLink:
		switch {
		case it.Native != nil:
			return it.Native.Link.URL
		case formatType.IsDirect():
			// In this case here have to be the advertisement link
			return it.Bid.AdMarkup
		}
	case adtype.ContentItemNotifyWinURL:
		if it.Bid != nil {
			return it.Bid.NURL
		}
	case adtype.ContentItemNotifyDisplayURL:
		if it.Bid != nil {
			return it.Bid.BURL
		}
	case types.FormatFieldTitle:
		if it.Native != nil {
			for _, asset := range it.Native.Assets {
				if asset.Title != nil {
					return asset.Title.Text
				}
			}
		}
	default:
		if it.Native != nil {
			for _, asset := range it.Native.Assets {
				if asset.Data != nil && asset.Data.Label == name {
					return asset.Data.Value
				}
			}
		}
	}
	return nil
}

// ContentFields from advertisement object
func (it *ResponseBidItem) ContentFields() map[string]any {
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
func (it *ResponseBidItem) ImpressionTrackerLinks() []string {
	if it.Native == nil {
		return nil
	}
	return it.Native.ImpTrackers
}

// ViewTrackerLinks returns traking links for view action
func (it *ResponseBidItem) ViewTrackerLinks() []string {
	return nil
}

// ClickTrackerLinks returns third-party tracker URLs to be fired on click of the URL
func (it *ResponseBidItem) ClickTrackerLinks() []string {
	if it.Native == nil {
		return nil
	}
	return it.Native.Link.ClickTrackers
}

// MainAsset from response
func (it *ResponseBidItem) MainAsset() *admodels.AdAsset {
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
func (it *ResponseBidItem) Assets() (assets admodels.AdAssets) {
	if it.assets != nil || it.Format().Config == nil {
		return it.assets
	}

	config := it.Format().Config
	for _, configAsset := range config.Assets {
		for _, asset := range it.Native.Assets {
			if asset.ID != configAsset.ID {
				continue
			}
			newAsset := &admodels.AdAsset{
				ID:   uint64(asset.ID),
				Name: configAsset.GetName(),
			}
			switch {
			case asset.Image != nil:
				newAsset.Path = asset.Image.URL
				newAsset.Type = types.AdAssetImageType
				newAsset.ContentType = ""
				newAsset.Width = asset.Image.Width
				newAsset.Height = asset.Image.Height
			// case asset.Video != nil:
			// 	newAsset.Path = asset.Video.URL
			// 	newAsset.Type = models.AdAssetVideoType
			default:
				// TODO error generation
			}
			it.assets = append(it.assets, newAsset)
			break
		}
	}
	return it.assets
}

// Format object model
func (it *ResponseBidItem) Format() *types.Format {
	if it == nil {
		return nil
	}
	return it.RespFormat
}

// PriorityFormatType from current Ad
func (it *ResponseBidItem) PriorityFormatType() types.FormatType {
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
func (it *ResponseBidItem) Impression() *adtype.Impression {
	return it.Imp
}

// ImpressionID unique code string
func (it *ResponseBidItem) ImpressionID() string {
	if it.Imp == nil {
		return ""
	}
	return it.Imp.ID
}

// ExtImpressionID unique code of RTB response
func (it *ResponseBidItem) ExtImpressionID() string {
	if it.Imp == nil {
		return ""
	}
	return it.Imp.ExternalID
}

// ExtTargetID of the external network
func (it *ResponseBidItem) ExtTargetID() string {
	return it.Imp.ExternalTargetID
}

// AdID returns the advertisement ID of the system
func (it *ResponseBidItem) AdID() uint64 {
	return 0
}

// AdCreativeID of the external advertisement
func (it *ResponseBidItem) AdCreativeID() string {
	if it == nil || it.Bid == nil {
		return ""
	}
	return it.Bid.CreativeID
}

// AccountID returns the account ID of the source
func (it *ResponseBidItem) AccountID() uint64 {
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
func (it *ResponseBidItem) CampaignID() uint64 {
	return 0
}

///////////////////////////////////////////////////////////////////////////////
// Price calculation methods
///////////////////////////////////////////////////////////////////////////////

// PricingModel of advertisement
// In case of RTB it can be CPM only
func (it *ResponseBidItem) PricingModel() types.PricingModel {
	return types.PricingModelCPM
}

// FixedPurchasePrice returns the fixed price of the action
func (it *ResponseBidItem) FixedPurchasePrice(action admodels.Action) billing.Money {
	return it.Imp.PurchasePrice(action)
}

// ECPM returns the effective cost per mille
func (it *ResponseBidItem) ECPM() billing.Money {
	if it == nil || it.Bid == nil {
		return 0
	}
	return it.PriceScope.ECPM
}

// PriceTestMode returns true if the price is in test mode
func (it *ResponseBidItem) PriceTestMode() bool { return false }

// Price for specific action if supported `click`, `lead`, `view`
// returns total price of the action
func (it *ResponseBidItem) Price(action admodels.Action) billing.Money {
	if it == nil || it.Bid == nil {
		return 0
	}
	price := it.PriceScope.PricePerAction(action)
	// price += adtype.PriceFactorFromList(removeFactors...).RemoveComission(price, it)
	return price
}

// BidPrice returns bid price for the external auction source.
// The current bid price will be adjusted according to the source correction factor and the commission share factor
func (it *ResponseBidItem) BidPrice() billing.Money {
	return it.PriceScope.BidPrice
}

// SetBidPrice value for external sources auction the system will pay
func (it *ResponseBidItem) SetBidPrice(bid billing.Money) error {
	if !it.PriceScope.SetBidPrice(bid, false) {
		return adtype.ErrNewAuctionBidIsHigherThenMaxBid
	}
	return nil
}

// InternalAuctionCPMBid value provides maximal possible price without any commission
// According to this value the system can choice the best item for the auction
func (it *ResponseBidItem) InternalAuctionCPMBid() billing.Money {
	// return it.AuctionCPMBid(adtype.AllPriceFactors)
	return price.CalculateInternalAuctionBid(it)
}

// PurchasePrice gives the price of view from external resource.
// The cost of this request for the system.
func (it *ResponseBidItem) PurchasePrice(action admodels.Action) billing.Money {
	if it == nil {
		return 0
	}
	// // Some sources can have the fixed price of buying
	// if pPrice := it.Imp.PurchasePrice(action); pPrice > 0 {
	// 	return pPrice
	// }
	// if len(removeFactors) == 0 {
	// 	removeFactors = []adtype.PriceFactor{^adtype.TargetReducePriceFactor}
	// }
	// switch action {
	// case admodels.ActionImpression: // Equal to admodels.ActionView
	// 	// As we buying from some source we can consider that we will loose approximately
	// 	// target gate reduce factor percent, but anyway price will be higher for X% of that descepancy
	// 	// to protect system from overspands
	// 	if it.Imp.Target.PricingModel().Or(it.PricingModel()).IsCPM() {
	// 		return it.AuctionCPMBid(removeFactors...) / 1000 // Price per One Impression
	// 	}
	// case admodels.ActionClick:
	// 	if it.Imp.Target.PricingModel().Or(it.PricingModel()).IsCPC() {
	// 		return it.Price(action, removeFactors...)
	// 	}
	// case admodels.ActionLead:
	// 	if it.Imp.Target.PricingModel().Or(it.PricingModel()).IsCPA() {
	// 		return it.Price(action, removeFactors...)
	// 	}
	// }
	// return 0
	return price.CalculatePurchasePrice(it, action)
}

// PotentialPrice wich can be received from source but was marked as descrepancy
func (it *ResponseBidItem) PotentialPrice(action admodels.Action) billing.Money {
	return price.CalculatePotentialPrice(it, action)
}

// FinalPrice for the action with all corrections and commissions
func (it *ResponseBidItem) FinalPrice(action admodels.Action) billing.Money {
	return price.CalculateFinalPrice(it, action)
}

// SetAuctionCPMBid value for external sources auction the system will pay
func (it *ResponseBidItem) SetAuctionCPMBid(price billing.Money, includeFactors ...adtype.PriceFactor) error {
	if len(includeFactors) > 0 {
		price += adtype.PriceFactorFromList(includeFactors...).AddComission(price, it)
	}
	if !it.PriceScope.SetBidPrice(price/1000, false) {
		return adtype.ErrNewAuctionBidIsHigherThenMaxBid
	}
	return nil
}

// Second campaigns
func (it *ResponseBidItem) Second() *adtype.SecondAd {
	return &it.SecondAd
}

///////////////////////////////////////////////////////////////////////////////
// Revenue share/comission methods
///////////////////////////////////////////////////////////////////////////////

// CommissionShareFactor which system get from publisher 0..1
func (it *ResponseBidItem) CommissionShareFactor() float64 {
	return it.Imp.CommissionShareFactor()
}

// SourceCorrectionFactor value for the source
func (it *ResponseBidItem) SourceCorrectionFactor() float64 {
	return it.Src.PriceCorrectionReduceFactor()
}

// TargetCorrectionFactor value for the target
func (it *ResponseBidItem) TargetCorrectionFactor() float64 {
	return it.Imp.Target.RevenueShareReduceFactor()
}

///////////////////////////////////////////////////////////////////////////////
// Other methods
///////////////////////////////////////////////////////////////////////////////

// RTBCategories of the advertisement
func (it *ResponseBidItem) RTBCategories() []string {
	if it.Bid == nil {
		return nil
	}
	return it.Bid.Cat
}

// IsDirect AD format
func (it *ResponseBidItem) IsDirect() bool {
	return it.Imp.IsDirect()
}

// IsBackup indicates whether the advertisement is a backup ad type.
func (it *ResponseBidItem) IsBackup() bool { return false }

// ActionURL for direct ADS
func (it *ResponseBidItem) ActionURL() string {
	return it.ActionLink
}

// Validate item
func (it *ResponseBidItem) Validate() error {
	if it.Src == nil || it.Req == nil || it.Imp == nil || it.Bid == nil {
		return adtype.ErrInvalidItemInitialisation
	}
	return it.Bid.Validate()
}

// Width of item
func (it *ResponseBidItem) Width() int {
	if it.Bid == nil {
		return 0
	}
	return it.Bid.W
}

// Height of item
func (it *ResponseBidItem) Height() int {
	if it.Bid == nil {
		return 0
	}
	return it.Bid.H
}

// Markup advertisement
func (it *ResponseBidItem) Markup() (string, error) {
	return "", nil
}

///////////////////////////////////////////////////////////////////////////////
// Context methods
///////////////////////////////////////////////////////////////////////////////

// Context value
func (it *ResponseBidItem) Context(ctx ...context.Context) context.Context {
	if len(ctx) > 0 {
		it.context = ctx[0]
	}
	return it.context
}

// Get ext field
func (it *ResponseBidItem) Get(key string) (res any) {
	if it.context == nil {
		return res
	}
	return it.context.Value(key)
}

var (
	_ adtype.ResponserItem = &ResponseBidItem{}
)
