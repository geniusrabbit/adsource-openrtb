//
// @project GeniusRabbit corelib 2017 - 2019, 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019, 2025
//

// Package banner implements the OpenRTB bid response item for banner ad formats.
// It handles HTML, iframe, and image banner creatives received from RTB sources,
// including interstitial XML markup parsing.
package banner

import (
	"context"
	"errors"
	"strings"

	"github.com/bsm/openrtb"
	"github.com/demdxx/gocast/v2"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/billing"
	"github.com/geniusrabbit/adcorelib/price"

	"github.com/geniusrabbit/adsource-openrtb/response/internal/interstitial"
)

// ErrInvalidAdContent is returned when the ad markup cannot be parsed into valid banner content.
var ErrInvalidAdContent = errors.New("invalid ad content")

// bannerFormatType infers the FormatType for a banner ad from its markup string.
// URLs and iframe markup are treated as proxy (FormatProxyType); everything else
// is treated as an inline HTML banner (FormatBannerType).
func bannerFormatType(markup string) types.FormatType {
	if strings.HasPrefix(markup, "http://") ||
		strings.HasPrefix(markup, "https://") ||
		(strings.HasPrefix(markup, "//") && !strings.ContainsAny(markup, "\n\t")) ||
		strings.Contains(markup, "<iframe") {
		return types.FormatProxyType
	}
	return types.FormatBannerType
}

// BannerInfo contains the parsed creative content for a banner advertisement.
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

// IsValid reports whether the banner info contains enough data to render an ad.
func (b *BannerInfo) IsValid() bool {
	return b.HTML != "" || b.IframeURL != "" || (b.LinkURL != "" && (b.ImageURL != "" || b.VideoURL != ""))
}

// ResponseBidItem is the bid response item for banner ad formats.
// It implements [adtype.ResponseItem] and wraps the raw OpenRTB bid together with
// parsed creative content, pricing, and tracking information.
type ResponseBidItem struct {
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

// New creates a ResponseBidItem for a banner bid. It parses the ad markup from
// the OpenRTB Bid and returns [ErrInvalidAdContent] if the markup cannot be
// converted into renderable content.
func New(req adtype.BidRequester, src adtype.Source, bid *openrtb.Bid, imp *adtype.Impression, format *types.Format) (*ResponseBidItem, error) {
	cpmPrice := billing.MoneyFloat(bid.Price)
	priceScope := price.PriceScopeImpression{
		MaxBidImpPrice: 0,
		BidImpPrice:    0,
		ImpPrice:       cpmPrice / 1000, // Convert CPM to per-impression price
		ECPM:           cpmPrice,
	}

	bidItem := &ResponseBidItem{
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

	switch {
	case strings.HasPrefix(bid.AdMarkup, "https://") || strings.HasPrefix(bid.AdMarkup, "http://"):
		bidItem.BannerInfo.IframeURL = bid.AdMarkup
	case strings.HasPrefix(bid.AdMarkup, "<?xml"):
		iframeURL, clickURL, imgURL, err := interstitial.ParseAdMarkup(bid.AdMarkup)
		if err != nil {
			return nil, err
		}
		bidItem.BannerInfo.IframeURL = iframeURL
		bidItem.BannerInfo.LinkURL = clickURL
		bidItem.BannerInfo.ImageURL = imgURL
	case strings.HasPrefix(bid.AdMarkup, "//"):
		bidItem.BannerInfo.IframeURL = bid.AdMarkup
	default:
		bidItem.BannerInfo.HTML = bid.AdMarkup
	}

	if !bidItem.BannerInfo.IsValid() {
		return nil, ErrInvalidAdContent
	}

	bidItem.PriceScope.MaxBidImpPrice = price.CalculatePurchasePrice(bidItem, adtype.ActionImpression)
	return bidItem, nil
}

// ID returns the unique identifier of the response item.
func (it *ResponseBidItem) ID() string {
	return it.ItemID
}

// Source returns the ad source associated with this bid.
func (it *ResponseBidItem) Source() adtype.Source {
	return it.Src
}

// NetworkName returns the network name for this source (always empty for RTB).
func (it *ResponseBidItem) NetworkName() string {
	return ""
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

// ContentFields returns a map of all populated content fields for this banner item.
func (it *ResponseBidItem) ContentFields() map[string]any {
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

// ImpressionTrackerLinks returns tracking links fired on impression.
func (it *ResponseBidItem) ImpressionTrackerLinks() []string {
	return it.BannerInfo.ImpTrackers
}

// ViewTrackerLinks returns tracking links fired on viewable impression.
func (it *ResponseBidItem) ViewTrackerLinks() []string {
	return it.BannerInfo.ViewTrackers
}

// ClickTrackerLinks returns third-party tracker URLs fired on click.
func (it *ResponseBidItem) ClickTrackerLinks() []string {
	return it.BannerInfo.ClickTrackers
}

// MainAsset returns the primary file asset (nil for banner format).
func (it *ResponseBidItem) MainAsset() *admodels.AdFileAsset {
	return nil
}

// Assets returns the list of file assets associated with this bid item.
func (it *ResponseBidItem) Assets() admodels.AdFileAssets {
	return nil
}

// Format returns the matched format object for this bid item.
func (it *ResponseBidItem) Format() *types.Format {
	if it == nil {
		return nil
	}
	return it.RespFormat
}

// PriorityFormatType returns the primary format type for this ad item.
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

// Impression returns the impression object associated with this bid item.
func (it *ResponseBidItem) Impression() *adtype.Impression {
	return it.Imp
}

// ImpressionID returns the unique impression identifier.
func (it *ResponseBidItem) ImpressionID() string {
	if it.Imp == nil {
		return ""
	}
	return it.Imp.ID
}

// ExtImpressionID returns the external (RTB) impression identifier.
func (it *ResponseBidItem) ExtImpressionID() string {
	if it.Imp == nil {
		return ""
	}
	return it.Imp.ExternalID
}

// ExtTargetID returns the external target identifier.
func (it *ResponseBidItem) ExtTargetID() string {
	return it.Imp.ExternalTargetID
}

// TargetCodename returns the codename of the target placement.
func (it *ResponseBidItem) TargetCodename() string {
	return it.Imp.TargetCodename()
}

// AdID returns the system advertisement ID (always empty for RTB).
func (it *ResponseBidItem) AdID() string {
	return ""
}

// CreativeID returns the creative ID from the RTB bid.
func (it *ResponseBidItem) CreativeID() string {
	if it == nil || it.Bid == nil {
		return ""
	}
	return it.Bid.CreativeID
}

// AccountID returns the account ID from the ad source.
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

// CampaignID returns the campaign ID (always 0 for RTB sources).
func (it *ResponseBidItem) CampaignID() uint64 {
	return 0
}

///////////////////////////////////////////////////////////////////////////////
// Price calculation methods
///////////////////////////////////////////////////////////////////////////////

// PricingModel returns CPM as the pricing model for all RTB banner bids.
func (it *ResponseBidItem) PricingModel() types.PricingModel {
	return types.PricingModelCPM
}

// FixedPurchasePrice returns the fixed purchase price for the given action from the impression.
func (it *ResponseBidItem) FixedPurchasePrice(action adtype.Action) billing.Money {
	return it.Imp.PurchasePrice(action)
}

// ECPM returns the effective cost per mille for this bid.
func (it *ResponseBidItem) ECPM() billing.Money {
	if it == nil || it.Bid == nil {
		return 0
	}
	return it.PriceScope.ECPM
}

// PriceTestMode always returns false for RTB banner bids.
func (it *ResponseBidItem) PriceTestMode() bool { return false }

// Price returns the total price for the given action (impression, click, lead, view).
func (it *ResponseBidItem) Price(action adtype.Action) billing.Money {
	if it == nil || it.Bid == nil {
		return 0
	}
	return it.PriceScope.PricePerAction(action)
}

// BidImpressionPrice returns the bid price that the system will pay for an impression.
func (it *ResponseBidItem) BidImpressionPrice() billing.Money {
	return it.PriceScope.BidImpPrice
}

// SetBidImpressionPrice sets the bid impression price. Returns an error if the new
// price exceeds the maximum allowed bid price.
func (it *ResponseBidItem) SetBidImpressionPrice(bid billing.Money) error {
	if !it.PriceScope.SetBidImpressionPrice(bid, false) {
		return adtype.ErrNewAuctionBidIsHigherThenMaxBid
	}
	return nil
}

// PrepareBidImpressionPrice adjusts the given price according to source correction
// and commission factors.
func (it *ResponseBidItem) PrepareBidImpressionPrice(p billing.Money) billing.Money {
	return it.PriceScope.PrepareBidImpressionPrice(p)
}

// InternalAuctionCPMBid returns the maximal possible price without any commission,
// used for internal auction ranking.
func (it *ResponseBidItem) InternalAuctionCPMBid() billing.Money {
	return price.CalculateInternalAuctionBid(it)
}

// PurchasePrice returns the actual cost of the given action for the system.
func (it *ResponseBidItem) PurchasePrice(action adtype.Action) billing.Money {
	return price.CalculatePurchasePrice(it, action)
}

// PotentialPrice returns the price that could have been received but was marked as discrepancy.
func (it *ResponseBidItem) PotentialPrice(action adtype.Action) billing.Money {
	return price.CalculatePotentialPrice(it, action)
}

// FinalPrice returns the price after all corrections and commissions for the given action.
func (it *ResponseBidItem) FinalPrice(action adtype.Action) billing.Money {
	return price.CalculateFinalPrice(it, action)
}

// Second returns the competitive second ad slot.
func (it *ResponseBidItem) Second() *adtype.SecondAd {
	return &it.SecondAd
}

///////////////////////////////////////////////////////////////////////////////
// Revenue share / commission methods
///////////////////////////////////////////////////////////////////////////////

// CommissionShareFactor returns the commission fraction (0..1) taken from the publisher.
func (it *ResponseBidItem) CommissionShareFactor() float64 {
	return it.Imp.CommissionShareFactor()
}

// SourceCorrectionFactor returns the price correction factor for this RTB source.
func (it *ResponseBidItem) SourceCorrectionFactor() float64 {
	return it.Src.PriceCorrectionReduceFactor()
}

// TargetCorrectionFactor returns the revenue-share reduction factor for the target.
func (it *ResponseBidItem) TargetCorrectionFactor() float64 {
	return it.Imp.Target.RevenueShareReduceFactor()
}

///////////////////////////////////////////////////////////////////////////////
// Other methods
///////////////////////////////////////////////////////////////////////////////

// RTBCategories returns the IAB content categories declared in the bid.
func (it *ResponseBidItem) RTBCategories() []string {
	if it.Bid == nil {
		return nil
	}
	return it.Bid.Cat
}

// IsDirect reports whether this is a direct ad format.
func (it *ResponseBidItem) IsDirect() bool {
	return it.Imp.IsDirect()
}

// IsBackup always returns false for banner bid items.
func (it *ResponseBidItem) IsBackup() bool { return false }

// ActionURL returns the click-through URL for the banner ad.
func (it *ResponseBidItem) ActionURL() string {
	return it.BannerInfo.LinkURL
}

// Validate checks that all required fields are populated.
func (it *ResponseBidItem) Validate() error {
	if it.Src == nil || it.Req == nil || it.Imp == nil || it.Bid == nil {
		return adtype.ErrInvalidItemInitialisation
	}
	return it.Bid.Validate()
}

// Width returns the creative width in pixels.
func (it *ResponseBidItem) Width() int {
	if it.Bid == nil {
		return 0
	}
	return it.Bid.W
}

// Height returns the creative height in pixels.
func (it *ResponseBidItem) Height() int {
	if it.Bid == nil {
		return 0
	}
	return it.Bid.H
}

// Markup returns the rendered ad markup (empty for banner; rendered by template engine).
func (it *ResponseBidItem) Markup() (string, error) {
	return "", nil
}

///////////////////////////////////////////////////////////////////////////////
// Context methods
///////////////////////////////////////////////////////////////////////////////

// Context gets or sets the context associated with this bid item.
func (it *ResponseBidItem) Context(ctx ...context.Context) context.Context {
	if len(ctx) > 0 {
		it.context = ctx[0]
	}
	return it.context
}

// Get retrieves a value from the item context by key.
func (it *ResponseBidItem) Get(key string) (res any) {
	if it.context == nil {
		return res
	}
	return it.context.Value(key)
}

var (
	_ adtype.ResponseItem = &ResponseBidItem{}
)
