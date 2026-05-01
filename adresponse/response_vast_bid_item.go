//
// @project GeniusRabbit corelib 2017 - 2019, 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019, 2025
//
// This file is part of the GeniusRabbit corelib project.
//
// VAST bid item for the ad response. This struct represents a bid item that contains information about the bid, impression, source, and other relevant data needed to process and display the advertisement in VAST format.
// The ResponseVASTBidItem struct implements the adtype.ResponseItem interface, which defines the methods required for handling ad response items in the system. This struct is specifically designed to handle VAST format bids and includes methods for retrieving content, tracking links, assets, pricing information, and other relevant data for processing the advertisement.
//
// VAST Example:
// ```xml
// <VAST version="4.0">
//   <Ad id="12345">
//     <InLine>
//       <AdTitle>Sample VAST Ad</AdTitle>
//       <Creatives>
//         <Creative>
//           <Linear>
//             <Duration>00:00:30</Duration>
//             <MediaFiles>
//               <MediaFile delivery="progressive" type="video/mp4" width="640" height="360">
//                 <![CDATA[https://example.com/video.mp4]]>
//               </MediaFile>
//             </MediaFiles>
//             <VideoClicks>
//               <ClickThrough><![CDATA[https://example.com/click]]></ClickThrough>
//             </VideoClicks>
//           </Linear>
//         </Creative>
//       </Creatives>
//     </InLine>
//   </Ad>
// </VAST>
//
// In this example, the VAST XML defines a single ad with an inline creative that includes a video media file and a click-through URL.
//
// Params:
// - VASTAdTagURI - The URI of the VAST ad tag, which is used to retrieve the VAST XML for the ad.
// - AdTitle - The title of the ad, which is used for display purposes.
// - Creatives - The creative elements of the ad, which include the media files and tracking information.
// - Duration - The duration of the video ad.
// - MediaFiles - The media files associated with the ad, including their delivery method, type, and dimensions.
// - VideoClicks - The click-through URLs for the video ad.
//
// The ResponseVASTBidItem struct provides methods to access this information and process the VAST bid accordingly.

package adresponse

import (
	"context"
	"strings"
	"time"

	"github.com/demdxx/gocast/v2"
	"github.com/demdxx/xtypes"
	"github.com/haxqer/vast"
	"go.uber.org/zap"

	"github.com/bsm/openrtb"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/billing"
	"github.com/geniusrabbit/adcorelib/context/ctxlogger"
	"github.com/geniusrabbit/adcorelib/price"
)

// ResponseVASTBidItem represents a bid item for VAST format in the ad response. It contains information about the bid, impression, source, and other relevant data needed to process and display the advertisement.
type ResponseVASTBidItem struct {
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

	// Tracking links for impression and click actions
	impressionTrackers []string
	clickTrackers      []string
	viewTrackers       []string

	assets  admodels.AdFileAssets
	context context.Context
}

func newResponseVASTBidItem(req adtype.BidRequester, src adtype.Source, bid *openrtb.Bid, imp *adtype.Impression, format *types.Format) (*ResponseVASTBidItem, error) {
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
	bidItem := &ResponseVASTBidItem{
		ItemID:     imp.ID,
		Src:        src,
		Req:        req,
		Imp:        imp,
		Bid:        bid,
		FormatType: types.FormatVideoType,
		RespFormat: format,
		PriceScope: priceScope,
	}

	// Handle video ad format (VAST)
	vastAd, err := unmarshalVAST([]byte(bid.AdMarkup))
	if err != nil {
		// Log VAST decoding failures
		ctxlogger.Get(req.Context()).Debug(
			"Failed to decode VAST markup",
			zap.String("markup", bid.AdMarkup),
			zap.Error(err),
		)
	}
	if err := validateVAST(vastAd); err != nil {
		// Log invalid VAST responses
		ctxlogger.Get(req.Context()).Debug(
			"Invalid VAST response",
			zap.String("markup", bid.AdMarkup),
			zap.Error(err),
		)
		return nil, err
	}

	// Set the bid impression price based on the bid price and impression
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

// ID of current response item (unique code of current response)
func (it *ResponseVASTBidItem) ID() string {
	return it.ItemID
}

// Source of response
func (it *ResponseVASTBidItem) Source() adtype.Source {
	return it.Src
}

// NetworkName by source
func (it *ResponseVASTBidItem) NetworkName() string {
	return ""
}

// ContentItemString from the ad
func (it *ResponseVASTBidItem) ContentItemString(name string) string {
	if val := it.ContentItem(name); val != nil {
		return gocast.Str(val)
	}
	return ""
}

// ContentItem returns the ad response data
func (it *ResponseVASTBidItem) ContentItem(name string) any {
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

// ContentFields from advertisement object
func (it *ResponseVASTBidItem) ContentFields() map[string]any {
	return nil
}

// ImpressionTrackerLinks returns traking links for impression action
func (it *ResponseVASTBidItem) ImpressionTrackerLinks() []string {
	return it.impressionTrackers
}

// ViewTrackerLinks returns traking links for view action
func (it *ResponseVASTBidItem) ViewTrackerLinks() []string {
	return it.viewTrackers
}

// ClickTrackerLinks returns third-party tracker URLs to be fired on click of the URL
func (it *ResponseVASTBidItem) ClickTrackerLinks() []string {
	return it.clickTrackers
}

// MainAsset from response
func (it *ResponseVASTBidItem) MainAsset() *admodels.AdFileAsset {
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
func (it *ResponseVASTBidItem) Assets() admodels.AdFileAssets {
	return it.assets
}

// Format object model
func (it *ResponseVASTBidItem) Format() *types.Format {
	if it == nil {
		return nil
	}
	return it.RespFormat
}

// PriorityFormatType from current Ad
func (it *ResponseVASTBidItem) PriorityFormatType() types.FormatType {
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
func (it *ResponseVASTBidItem) Impression() *adtype.Impression {
	return it.Imp
}

// ImpressionID unique code string
func (it *ResponseVASTBidItem) ImpressionID() string {
	if it.Imp == nil {
		return ""
	}
	return it.Imp.ID
}

// ExtImpressionID unique code of RTB response
func (it *ResponseVASTBidItem) ExtImpressionID() string {
	if it.Imp == nil {
		return ""
	}
	return it.Imp.ExternalID
}

// ExtTargetID of the external network
func (it *ResponseVASTBidItem) ExtTargetID() string {
	return it.Imp.ExternalTargetID
}

// TargetCodename of the target placement codename
func (it *ResponseVASTBidItem) TargetCodename() string {
	return it.Imp.TargetCodename()
}

// AdID returns the advertisement ID of the system
func (it *ResponseVASTBidItem) AdID() string {
	return ""
}

// CreativeID of the external advertisement
func (it *ResponseVASTBidItem) CreativeID() string {
	if it == nil || it.Bid == nil {
		return ""
	}
	return it.Bid.CreativeID
}

// AccountID returns the account ID of the source
func (it *ResponseVASTBidItem) AccountID() uint64 {
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
func (it *ResponseVASTBidItem) CampaignID() uint64 {
	return 0
}

///////////////////////////////////////////////////////////////////////////////
// Price calculation methods
///////////////////////////////////////////////////////////////////////////////

// PricingModel of advertisement
// In case of RTB it can be CPM only
func (it *ResponseVASTBidItem) PricingModel() types.PricingModel {
	return types.PricingModelCPM
}

// FixedPurchasePrice returns the fixed price of the action
func (it *ResponseVASTBidItem) FixedPurchasePrice(action adtype.Action) billing.Money {
	return it.Imp.PurchasePrice(action)
}

// ECPM returns the effective cost per mille
func (it *ResponseVASTBidItem) ECPM() billing.Money {
	if it == nil || it.Bid == nil {
		return 0
	}
	return it.PriceScope.ECPM
}

// PriceTestMode returns true if the price is in test mode
func (it *ResponseVASTBidItem) PriceTestMode() bool { return false }

// Price for specific action if supported `click`, `lead`, `view`
// returns total price of the action
func (it *ResponseVASTBidItem) Price(action adtype.Action) billing.Money {
	if it == nil || it.Bid == nil {
		return 0
	}
	price := it.PriceScope.PricePerAction(action)
	return price
}

// BidViewPrice returns bid price for the external auction source.
// The current bid price will be adjusted according to the source correction factor and the commission share factor
func (it *ResponseVASTBidItem) BidImpressionPrice() billing.Money {
	return it.PriceScope.BidImpPrice
}

// SetBidImpressionPrice value for external sources auction the system will pay
func (it *ResponseVASTBidItem) SetBidImpressionPrice(bid billing.Money) error {
	if !it.PriceScope.SetBidImpressionPrice(bid, false) {
		return adtype.ErrNewAuctionBidIsHigherThenMaxBid
	}
	return nil
}

// PrepareBidImpressionPrice prepares the price for the action
// The price is adjusted according to the source correction factor and the commission share factor
func (it *ResponseVASTBidItem) PrepareBidImpressionPrice(price billing.Money) billing.Money {
	return it.PriceScope.PrepareBidImpressionPrice(price)
}

// InternalAuctionCPMBid value provides maximal possible price without any commission
// According to this value the system can choice the best item for the auction
func (it *ResponseVASTBidItem) InternalAuctionCPMBid() billing.Money {
	return price.CalculateInternalAuctionBid(it)
}

// PurchasePrice gives the price of view from external resource.
// The cost of this request for the system.
func (it *ResponseVASTBidItem) PurchasePrice(action adtype.Action) billing.Money {
	return price.CalculatePurchasePrice(it, action)
}

// PotentialPrice wich can be received from source but was marked as descrepancy
func (it *ResponseVASTBidItem) PotentialPrice(action adtype.Action) billing.Money {
	return price.CalculatePotentialPrice(it, action)
}

// FinalPrice for the action with all corrections and commissions
func (it *ResponseVASTBidItem) FinalPrice(action adtype.Action) billing.Money {
	return price.CalculateFinalPrice(it, action)
}

// Second campaigns
func (it *ResponseVASTBidItem) Second() *adtype.SecondAd {
	return &it.SecondAd
}

///////////////////////////////////////////////////////////////////////////////
// Revenue share/comission methods
///////////////////////////////////////////////////////////////////////////////

// CommissionShareFactor which system get from publisher 0..1
func (it *ResponseVASTBidItem) CommissionShareFactor() float64 {
	return it.Imp.CommissionShareFactor()
}

// SourceCorrectionFactor value for the source
func (it *ResponseVASTBidItem) SourceCorrectionFactor() float64 {
	return it.Src.PriceCorrectionReduceFactor()
}

// TargetCorrectionFactor value for the target
func (it *ResponseVASTBidItem) TargetCorrectionFactor() float64 {
	return it.Imp.Target.RevenueShareReduceFactor()
}

///////////////////////////////////////////////////////////////////////////////
// Other methods
///////////////////////////////////////////////////////////////////////////////

// RTBCategories of the advertisement
func (it *ResponseVASTBidItem) RTBCategories() []string {
	if it.Bid == nil {
		return nil
	}
	return it.Bid.Cat
}

// IsDirect AD format
func (it *ResponseVASTBidItem) IsDirect() bool {
	return false
}

// IsBackup indicates whether the advertisement is a backup ad type.
func (it *ResponseVASTBidItem) IsBackup() bool { return false }

// ActionURL for direct ADS
func (it *ResponseVASTBidItem) ActionURL() string {
	if it.VAST.Ads[0].InLine != nil {
		return it.VAST.Ads[0].InLine.Creatives[0].Linear.VideoClicks.ClickThroughs[0].URI
	}
	return it.VAST.Ads[0].Wrapper.Creatives[0].Linear.VideoClicks.ClickThroughs[0].URI
}

// Validate item
func (it *ResponseVASTBidItem) Validate() error {
	if it.Src == nil || it.Req == nil || it.Imp == nil || it.Bid == nil {
		return adtype.ErrInvalidItemInitialisation
	}
	return it.Bid.Validate()
}

// Width of item
func (it *ResponseVASTBidItem) Width() int {
	if it.Bid == nil {
		return 0
	}
	return it.Bid.W
}

// Height of item
func (it *ResponseVASTBidItem) Height() int {
	if it.Bid == nil {
		return 0
	}
	return it.Bid.H
}

// Markup advertisement
func (it *ResponseVASTBidItem) Markup() (string, error) {
	return "", nil
}

///////////////////////////////////////////////////////////////////////////////
// Context methods
///////////////////////////////////////////////////////////////////////////////

// Context value
func (it *ResponseVASTBidItem) Context(ctx ...context.Context) context.Context {
	if len(ctx) > 0 {
		it.context = ctx[0]
	}
	return it.context
}

// Get ext field
func (it *ResponseVASTBidItem) Get(key string) (res any) {
	if it.context == nil {
		return res
	}
	return it.context.Value(key)
}

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
	_ adtype.ResponseItem = &ResponseVASTBidItem{}
)
