//
// @project GeniusRabbit corelib 2017 - 2019, 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019, 2025
//

// Package vast implements the OpenRTB bid response item for VAST video ad formats.
// VAST (Video Ad Serving Template) bids deliver an XML document describing
// video creatives, tracking events, and click-through URLs.
//
// VAST Example:
//
//	<VAST version="4.0">
//	  <Ad id="12345">
//	    <InLine>
//	      <AdTitle>Sample VAST Ad</AdTitle>
//	      <Creatives>
//	        <Creative>
//	          <Linear>
//	            <Duration>00:00:30</Duration>
//	            <MediaFiles>
//	              <MediaFile delivery="progressive" type="video/mp4" width="640" height="360">
//	                <![CDATA[https://example.com/video.mp4]]>
//	              </MediaFile>
//	            </MediaFiles>
//	            <VideoClicks>
//	              <ClickThrough><![CDATA[https://example.com/click]]></ClickThrough>
//	            </VideoClicks>
//	          </Linear>
//	        </Creative>
//	      </Creatives>
//	    </InLine>
//	  </Ad>
//	</VAST>
package vast

import (
	"context"
	"strings"
	"time"

	"github.com/bsm/openrtb"
	"github.com/demdxx/gocast/v2"
	"github.com/demdxx/xtypes"
	"github.com/haxqer/vast"
	"go.uber.org/zap"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/billing"
	"github.com/geniusrabbit/adcorelib/context/ctxlogger"
	"github.com/geniusrabbit/adcorelib/price"
)

// ResponseBidItem is the bid response item for VAST video ad formats.
// It implements [adtype.ResponseItem] and provides access to the decoded VAST
// document, media assets, trackers, and pricing information.
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
	Bid  *openrtb.Bid `json:"bid,omitempty"`
	VAST *vast.VAST   `json:"vast,omitempty"`

	PriceScope price.PriceScopeImpression `json:"price_scope,omitempty"`

	// Competitive second AD
	SecondAd adtype.SecondAd `json:"second_ad,omitempty"`

	Data map[string]any `json:"data,omitempty"`

	// Tracking links for impression, view and click actions
	impressionTrackers []string
	clickTrackers      []string
	viewTrackers       []string

	assets  admodels.AdFileAssets
	context context.Context
}

// New creates a ResponseBidItem for a VAST bid. It decodes and validates the
// XML VAST markup from the OpenRTB Bid, extracts tracking URLs and media assets.
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
		FormatType: types.FormatVideoType,
		RespFormat: format,
		PriceScope: priceScope,
	}

	vastAd, err := unmarshalVAST([]byte(bid.AdMarkup))
	if err != nil {
		ctxlogger.Get(req.Context()).Debug(
			"Failed to decode VAST markup",
			zap.String("markup", bid.AdMarkup),
			zap.Error(err),
		)
	}
	if err := validateVAST(vastAd); err != nil {
		ctxlogger.Get(req.Context()).Debug(
			"Invalid VAST response",
			zap.String("markup", bid.AdMarkup),
			zap.Error(err),
		)
		return nil, err
	}

	bidItem.VAST = vastAd
	bidItem.PriceScope.MaxBidImpPrice = price.CalculatePurchasePrice(bidItem, adtype.ActionImpression)

	// Extract tracking links from the VAST response
	if vastAd.Ads[0].InLine != nil {
		bidItem.impressionTrackers = xtypes.SliceApply(
			vastAd.Ads[0].InLine.Impressions,
			func(impression vast.Impression) string { return impression.URI })
		bidItem.viewTrackers = xtypes.SliceApply(
			vastAd.Ads[0].InLine.ViewableImpression.Viewable,
			func(click vast.CDATAString) string { return click.CDATA })
		bidItem.clickTrackers = xtypes.SliceApply(
			vastAd.Ads[0].InLine.Creatives[0].Linear.VideoClicks.ClickTrackings,
			func(click vast.VideoClick) string { return click.URI })
		bidItem.clickTrackers = append(bidItem.clickTrackers, xtypes.SliceApply(
			vastAd.Ads[0].InLine.Creatives[0].Linear.VideoClicks.CustomClicks,
			func(click vast.VideoClick) string { return click.URI })...)
	} else if vastAd.Ads[0].Wrapper != nil {
		bidItem.impressionTrackers = xtypes.SliceApply(
			vastAd.Ads[0].Wrapper.Impressions,
			func(impression vast.Impression) string { return impression.URI })
		bidItem.viewTrackers = xtypes.SliceApply(
			vastAd.Ads[0].Wrapper.ViewableImpression.Viewable,
			func(click vast.CDATAString) string { return click.CDATA })
		bidItem.clickTrackers = xtypes.SliceApply(
			vastAd.Ads[0].Wrapper.Creatives[0].Linear.VideoClicks.ClickTrackings,
			func(click vast.VideoClick) string { return click.URI })
		bidItem.clickTrackers = append(bidItem.clickTrackers, xtypes.SliceApply(
			vastAd.Ads[0].Wrapper.Creatives[0].Linear.VideoClicks.CustomClicks,
			func(click vast.VideoClick) string { return click.URI })...)
	}

	// Extract media assets from the VAST response
	if vastAd.Ads[0].InLine != nil {
		for _, creative := range vastAd.Ads[0].InLine.Creatives {
			if creative.Linear == nil || creative.Linear.MediaFiles == nil {
				continue
			}
			mediaAssets := fileAssetsFromMediaFiles(creative.Linear.MediaFiles.MediaFile,
				int(time.Duration(creative.Linear.Duration).Seconds()))
			bidItem.assets = append(bidItem.assets, mediaAssets...)
			if creative.Linear.Icons != nil && creative.Linear.Icons.Icon != nil {
				iconAssets := iconAssetsFromIcons(*creative.Linear.Icons.Icon)
				bidItem.assets = append(bidItem.assets, iconAssets...)
			}
		}
	} else if vastAd.Ads[0].Wrapper != nil {
		bidItem.assets = admodels.AdFileAssets{
			{
				ID:          999, // Arbitrary ID for the main VAST tag asset
				Name:        "vast_tag",
				URL:         vastAd.Ads[0].Wrapper.VASTAdTagURI.CDATA,
				Type:        types.AdFileAssetVASTTagType,
				ContentType: "application/xml",
			},
		}
		for _, creative := range vastAd.Ads[0].Wrapper.Creatives {
			if creative.Linear == nil || creative.Linear.Icons == nil {
				continue
			}
			if creative.Linear.Icons != nil && creative.Linear.Icons.Icon != nil {
				iconAssets := iconAssetsFromIcons(*creative.Linear.Icons.Icon)
				bidItem.assets = append(bidItem.assets, iconAssets...)
			}
		}
	}

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
	if it.Data != nil {
		return it.Data[name]
	}

	switch name {
	case adtype.ContentItemLink:
		if it.VAST.Ads[0].InLine != nil {
			for _, creative := range it.VAST.Ads[0].InLine.Creatives {
				if creative.Linear != nil {
					return creative.Linear.VideoClicks.ClickThroughs[0].URI
				}
			}
		} else if it.VAST.Ads[0].Wrapper != nil {
			for _, creative := range it.VAST.Ads[0].Wrapper.Creatives {
				if creative.Linear != nil {
					return creative.Linear.VideoClicks.ClickThroughs[0].URI
				}
			}
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
		if it.VAST.Ads[0].InLine != nil {
			return it.VAST.Ads[0].InLine.AdTitle.CDATA
		}
	case types.FormatFieldDescription:
		if it.VAST.Ads[0].InLine != nil {
			return it.VAST.Ads[0].InLine.Description.CDATA
		}
	}
	return nil
}

// ContentFields returns a map of all populated content fields (nil for VAST format).
func (it *ResponseBidItem) ContentFields() map[string]any {
	return nil
}

// ImpressionTrackerLinks returns tracking links fired on impression.
func (it *ResponseBidItem) ImpressionTrackerLinks() []string {
	return it.impressionTrackers
}

// ViewTrackerLinks returns tracking links fired on viewable impression.
func (it *ResponseBidItem) ViewTrackerLinks() []string {
	return it.viewTrackers
}

// ClickTrackerLinks returns third-party tracker URLs fired on click.
func (it *ResponseBidItem) ClickTrackerLinks() []string {
	return it.clickTrackers
}

// MainAsset returns the primary file asset, matched by the format configuration.
func (it *ResponseBidItem) MainAsset() *admodels.AdFileAsset {
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

// Assets returns the list of file assets associated with this bid item.
func (it *ResponseBidItem) Assets() admodels.AdFileAssets {
	return it.assets
}

// Format returns the matched format object for this bid item.
func (it *ResponseBidItem) Format() *types.Format {
	if it == nil {
		return nil
	}
	return it.RespFormat
}

// PriorityFormatType returns FormatVideoType for VAST bids when set; otherwise
// derives the type from the impression format types.
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

// PricingModel returns CPM as the pricing model for all RTB VAST bids.
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

// PriceTestMode always returns false for RTB VAST bids.
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

// InternalAuctionCPMBid returns the maximal possible price without any commission.
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

// IsDirect always returns false for VAST bid items.
func (it *ResponseBidItem) IsDirect() bool {
	return false
}

// IsBackup always returns false for VAST bid items.
func (it *ResponseBidItem) IsBackup() bool { return false }

// ActionURL returns the primary video click-through URL from the VAST document.
func (it *ResponseBidItem) ActionURL() string {
	if it.VAST.Ads[0].InLine != nil {
		return it.VAST.Ads[0].InLine.Creatives[0].Linear.VideoClicks.ClickThroughs[0].URI
	}
	return it.VAST.Ads[0].Wrapper.Creatives[0].Linear.VideoClicks.ClickThroughs[0].URI
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

// Markup returns the rendered ad markup (empty for VAST; rendered by template engine).
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

// fileAssetsFromMediaFiles converts a slice of VAST [vast.MediaFile] descriptors
// into [admodels.AdFileAssets], inferring asset type from the MIME type prefix.
func fileAssetsFromMediaFiles(mediaFiles []vast.MediaFile, duration int) admodels.AdFileAssets {
	assets := make(admodels.AdFileAssets, 0, len(mediaFiles))
	for i, mediaFile := range mediaFiles {
		name := "main"
		if i > 0 {
			name = "media-" + gocast.Str(i)
		}
		asset := &admodels.AdFileAsset{
			Name:        name,
			ExternalID:  mediaFile.ID,
			URL:         mediaFile.URI,
			ContentType: mediaFile.Type,
			Width:       mediaFile.Width,
			Height:      mediaFile.Height,
			Duration:    duration,
		}
		if strings.HasPrefix(mediaFile.Type, "video/") {
			asset.Type = types.AdFileAssetVideoType
		} else if strings.HasPrefix(mediaFile.Type, "image/") {
			asset.Type = types.AdFileAssetImageType
		}
		assets = append(assets, asset)
	}
	return assets
}

// iconAssetsFromIcons converts a slice of VAST [vast.Icon] objects into
// [admodels.AdFileAssets] with the image asset type.
func iconAssetsFromIcons(icons []vast.Icon) admodels.AdFileAssets {
	assets := make(admodels.AdFileAssets, 0, len(icons))
	for i, icon := range icons {
		name := "icon"
		if i > 0 {
			name = "icon-" + gocast.Str(i)
		}
		asset := &admodels.AdFileAsset{
			Name:        name,
			URL:         icon.StaticResource.URI,
			AltText:     icon.AltText,
			ContentType: icon.StaticResource.CreativeType,
			Width:       icon.Width,
			Height:      icon.Height,
			Type:        types.AdFileAssetImageType,
		}
		assets = append(assets, asset)
	}
	return assets
}

var (
	_ adtype.ResponseItem = &ResponseBidItem{}
)
