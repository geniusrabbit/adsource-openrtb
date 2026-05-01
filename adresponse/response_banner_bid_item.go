//
// @project GeniusRabbit corelib 2017 - 2019, 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019, 2025
//

package adresponse

import (
	"context"
	"strings"

	"github.com/bsm/openrtb"
	"github.com/demdxx/gocast/v2"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/billing"
	"github.com/geniusrabbit/adcorelib/price"
)

// BannerInfo contains information about banner advertisement
type BannerInfo struct {
	Title         string   `json:"title,omitempty"`
	HTML          string   `json:"html,omitempty"`
	IframeURL     string   `json:"iframe_url,omitempty"`
	ImageURL      string   `json:"image_url,omitempty"`
	VideoURL      string   `json:"video_url,omitempty"`
	Width         int      `json:"width,omitempty"`
	Height        int      `json:"height,omitempty"`
	WRatio        int      `json:"wratio,omitempty"`
	HRatio        int      `json:"hratio,omitempty"`
	LinkURL       string   `json:"link_url,omitempty"`
	ImpTrackers   []string `json:"imp_trackers,omitempty"`
	ViewTrackers  []string `json:"view_trackers,omitempty"`
	ClickTrackers []string `json:"click_trackers,omitempty"`
}

func (b *BannerInfo) IsValid() bool {
	return b.HTML != "" || b.IframeURL != "" || (b.LinkURL != "" && (b.ImageURL != "" || b.VideoURL != ""))
}

// ResponseBannerBidItem value
type ResponseBannerBidItem struct {
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
	BannerInfo BannerInfo   `json:"banner_info"`

	PriceScope price.PriceScopeImpression `json:"price_scope,omitempty"`

	// Competitive second AD
	SecondAd adtype.SecondAd `json:"second_ad,omitempty"`

	assets  admodels.AdFileAssets `json:"-"`
	context context.Context       `json:"-"`
}

func newResponseBannerBidItem(req adtype.BidRequester, src adtype.Source, bid *openrtb.Bid, imp *adtype.Impression, format *types.Format) (*ResponseBannerBidItem, error) {
	// Calculate the bid price and set up the price scope for the bid item
	cpmPrice := billing.MoneyFloat(bid.Price)

	// Set the bid item properties
	priceScope := price.PriceScopeImpression{
		MaxBidImpPrice: 0,
		BidImpPrice:    0,
		ImpPrice:       cpmPrice / 1000, // Convert from micros (CPM) to actual price
		ECPM:           cpmPrice,        // Original eCPM price
	}

	// Create bid item for banner format
	bidItem := &ResponseBannerBidItem{
		ItemID:     imp.ID,
		Src:        src,
		Req:        req,
		Imp:        imp,
		Bid:        bid,
		FormatType: bannerFormatType(bid.AdMarkup),
		RespFormat: format,
		PriceScope: priceScope,
		BannerInfo: BannerInfo{
			Width:  bid.W,
			Height: bid.H,
			WRatio: bid.WRatio,
			HRatio: bid.HRatio,
		},
	}

	// Determine the content of the banner ad based on the ad markup
	if strings.HasPrefix(bid.AdMarkup, "https://") || strings.HasPrefix(bid.AdMarkup, "http://") {
		bidItem.BannerInfo.IframeURL = bid.AdMarkup
	} else if strings.HasPrefix(bid.AdMarkup, "<?xml") {
		iframeURL, clickURL, imgURL, err := parseInterstitialAdMarkup(bid.AdMarkup)
		if err != nil {
			return nil, err
		}
		bidItem.BannerInfo.IframeURL = iframeURL
		bidItem.BannerInfo.LinkURL = clickURL
		bidItem.BannerInfo.ImageURL = imgURL
	} else {
		if strings.HasPrefix(bid.AdMarkup, "//") {
			bidItem.BannerInfo.IframeURL = bid.AdMarkup
		} else {
			bidItem.BannerInfo.HTML = bid.AdMarkup
		}
	}

	// Validate the banner information
	if !bidItem.BannerInfo.IsValid() {
		return nil, ErrInvalidAdContent
	}

	// Set the bid impression price based on the bid price and impression
	bidItem.PriceScope.MaxBidImpPrice = price.CalculatePurchasePrice(bidItem, adtype.ActionImpression)

	return bidItem, nil
}

// ID of current response item (unique code of current response)
func (it *ResponseBannerBidItem) ID() string {
	return it.ItemID
}

// Source of response
func (it *ResponseBannerBidItem) Source() adtype.Source {
	return it.Src
}

// NetworkName by source
func (it *ResponseBannerBidItem) NetworkName() string {
	return ""
}

// ContentItemString from the ad
func (it *ResponseBannerBidItem) ContentItemString(name string) string {
	if val := it.ContentItem(name); val != nil {
		return gocast.Str(val)
	}
	return ""
}

// ContentItem returns the ad response data
func (it *ResponseBannerBidItem) ContentItem(name string) any {
	switch name {
	case adtype.ContentItemIFrameURL:
		return it.BannerInfo.IframeURL
	case adtype.ContentItemContent:
		return it.BannerInfo.HTML
	case adtype.ContentItemNotifyWinURL:
		if it.Bid != nil {
			return it.Bid.NURL
		}
	case adtype.ContentItemNotifyDisplayURL:
		if it.Bid != nil {
			return it.Bid.BURL
		}
	case types.FormatFieldTitle:
		return it.BannerInfo.Title
	}
	return nil
}

// ContentFields from advertisement object
func (it *ResponseBannerBidItem) ContentFields() map[string]any {
	if it.Format().Config == nil {
		return nil
	}
	fields := map[string]any{}
	if it.BannerInfo.IframeURL != "" {
		fields[adtype.ContentItemIFrameURL] = it.BannerInfo.IframeURL
	}
	if it.BannerInfo.HTML != "" {
		fields[adtype.ContentItemContent] = it.BannerInfo.HTML
	}
	if it.BannerInfo.Title != "" {
		fields[types.FormatFieldTitle] = it.BannerInfo.Title
	}
	return fields
}

// ImpressionTrackerLinks returns traking links for impression action
func (it *ResponseBannerBidItem) ImpressionTrackerLinks() []string {
	return it.BannerInfo.ImpTrackers
}

// ViewTrackerLinks returns traking links for view action
func (it *ResponseBannerBidItem) ViewTrackerLinks() []string {
	return it.BannerInfo.ViewTrackers
}

// ClickTrackerLinks returns third-party tracker URLs to be fired on click of the URL
func (it *ResponseBannerBidItem) ClickTrackerLinks() []string {
	return it.BannerInfo.ClickTrackers
}

// MainAsset from response
func (it *ResponseBannerBidItem) MainAsset() *admodels.AdFileAsset {
	return nil
}

// Assets returns list of the advertisement
func (it *ResponseBannerBidItem) Assets() admodels.AdFileAssets {
	return nil
}

// Format object model
func (it *ResponseBannerBidItem) Format() *types.Format {
	if it == nil {
		return nil
	}
	return it.RespFormat
}

// PriorityFormatType from current Ad
func (it *ResponseBannerBidItem) PriorityFormatType() types.FormatType {
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
func (it *ResponseBannerBidItem) Impression() *adtype.Impression {
	return it.Imp
}

// ImpressionID unique code string
func (it *ResponseBannerBidItem) ImpressionID() string {
	if it.Imp == nil {
		return ""
	}
	return it.Imp.ID
}

// ExtImpressionID unique code of RTB response
func (it *ResponseBannerBidItem) ExtImpressionID() string {
	if it.Imp == nil {
		return ""
	}
	return it.Imp.ExternalID
}

// ExtTargetID of the external network
func (it *ResponseBannerBidItem) ExtTargetID() string {
	return it.Imp.ExternalTargetID
}

// TargetCodename of the target placement codename
func (it *ResponseBannerBidItem) TargetCodename() string {
	return it.Imp.TargetCodename()
}

// AdID returns the advertisement ID of the system
func (it *ResponseBannerBidItem) AdID() string {
	return ""
}

// CreativeID of the external advertisement
func (it *ResponseBannerBidItem) CreativeID() string {
	if it == nil || it.Bid == nil {
		return ""
	}
	return it.Bid.CreativeID
}

// AccountID returns the account ID of the source
func (it *ResponseBannerBidItem) AccountID() uint64 {
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
func (it *ResponseBannerBidItem) CampaignID() uint64 {
	return 0
}

///////////////////////////////////////////////////////////////////////////////
// Price calculation methods
///////////////////////////////////////////////////////////////////////////////

// PricingModel of advertisement
// In case of RTB it can be CPM only
func (it *ResponseBannerBidItem) PricingModel() types.PricingModel {
	return types.PricingModelCPM
}

// FixedPurchasePrice returns the fixed price of the action
func (it *ResponseBannerBidItem) FixedPurchasePrice(action adtype.Action) billing.Money {
	return it.Imp.PurchasePrice(action)
}

// ECPM returns the effective cost per mille
func (it *ResponseBannerBidItem) ECPM() billing.Money {
	if it == nil || it.Bid == nil {
		return 0
	}
	return it.PriceScope.ECPM
}

// PriceTestMode returns true if the price is in test mode
func (it *ResponseBannerBidItem) PriceTestMode() bool { return false }

// Price for specific action if supported `click`, `lead`, `view`
// returns total price of the action
func (it *ResponseBannerBidItem) Price(action adtype.Action) billing.Money {
	if it == nil || it.Bid == nil {
		return 0
	}
	price := it.PriceScope.PricePerAction(action)
	return price
}

// BidViewPrice returns bid price for the external auction source.
// The current bid price will be adjusted according to the source correction factor and the commission share factor
func (it *ResponseBannerBidItem) BidImpressionPrice() billing.Money {
	return it.PriceScope.BidImpPrice
}

// SetBidImpressionPrice value for external sources auction the system will pay
func (it *ResponseBannerBidItem) SetBidImpressionPrice(bid billing.Money) error {
	if !it.PriceScope.SetBidImpressionPrice(bid, false) {
		return adtype.ErrNewAuctionBidIsHigherThenMaxBid
	}
	return nil
}

// PrepareBidImpressionPrice prepares the price for the action
// The price is adjusted according to the source correction factor and the commission share factor
func (it *ResponseBannerBidItem) PrepareBidImpressionPrice(price billing.Money) billing.Money {
	return it.PriceScope.PrepareBidImpressionPrice(price)
}

// InternalAuctionCPMBid value provides maximal possible price without any commission
// According to this value the system can choice the best item for the auction
func (it *ResponseBannerBidItem) InternalAuctionCPMBid() billing.Money {
	return price.CalculateInternalAuctionBid(it)
}

// PurchasePrice gives the price of view from external resource.
// The cost of this request for the system.
func (it *ResponseBannerBidItem) PurchasePrice(action adtype.Action) billing.Money {
	return price.CalculatePurchasePrice(it, action)
}

// PotentialPrice wich can be received from source but was marked as descrepancy
func (it *ResponseBannerBidItem) PotentialPrice(action adtype.Action) billing.Money {
	return price.CalculatePotentialPrice(it, action)
}

// FinalPrice for the action with all corrections and commissions
func (it *ResponseBannerBidItem) FinalPrice(action adtype.Action) billing.Money {
	return price.CalculateFinalPrice(it, action)
}

// Second campaigns
func (it *ResponseBannerBidItem) Second() *adtype.SecondAd {
	return &it.SecondAd
}

///////////////////////////////////////////////////////////////////////////////
// Revenue share/comission methods
///////////////////////////////////////////////////////////////////////////////

// CommissionShareFactor which system get from publisher 0..1
func (it *ResponseBannerBidItem) CommissionShareFactor() float64 {
	return it.Imp.CommissionShareFactor()
}

// SourceCorrectionFactor value for the source
func (it *ResponseBannerBidItem) SourceCorrectionFactor() float64 {
	return it.Src.PriceCorrectionReduceFactor()
}

// TargetCorrectionFactor value for the target
func (it *ResponseBannerBidItem) TargetCorrectionFactor() float64 {
	return it.Imp.Target.RevenueShareReduceFactor()
}

///////////////////////////////////////////////////////////////////////////////
// Other methods
///////////////////////////////////////////////////////////////////////////////

// RTBCategories of the advertisement
func (it *ResponseBannerBidItem) RTBCategories() []string {
	if it.Bid == nil {
		return nil
	}
	return it.Bid.Cat
}

// IsDirect AD format
func (it *ResponseBannerBidItem) IsDirect() bool {
	return it.Imp.IsDirect()
}

// IsBackup indicates whether the advertisement is a backup ad type.
func (it *ResponseBannerBidItem) IsBackup() bool { return false }

// ActionURL for direct ADS
func (it *ResponseBannerBidItem) ActionURL() string {
	return it.BannerInfo.LinkURL
}

// Validate item
func (it *ResponseBannerBidItem) Validate() error {
	if it.Src == nil || it.Req == nil || it.Imp == nil || it.Bid == nil {
		return adtype.ErrInvalidItemInitialisation
	}
	return it.Bid.Validate()
}

// Width of item
func (it *ResponseBannerBidItem) Width() int {
	if it.Bid == nil {
		return 0
	}
	return it.Bid.W
}

// Height of item
func (it *ResponseBannerBidItem) Height() int {
	if it.Bid == nil {
		return 0
	}
	return it.Bid.H
}

// Markup advertisement
func (it *ResponseBannerBidItem) Markup() (string, error) {
	return "", nil
}

///////////////////////////////////////////////////////////////////////////////
// Context methods
///////////////////////////////////////////////////////////////////////////////

// Context value
func (it *ResponseBannerBidItem) Context(ctx ...context.Context) context.Context {
	if len(ctx) > 0 {
		it.context = ctx[0]
	}
	return it.context
}

// Get ext field
func (it *ResponseBannerBidItem) Get(key string) (res any) {
	if it.context == nil {
		return res
	}
	return it.context.Value(key)
}

var (
	_ adtype.ResponseItem = &ResponseBannerBidItem{}
)
