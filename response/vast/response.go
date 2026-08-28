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
	"time"

	"github.com/bsm/openrtb"
	"github.com/demdxx/gocast/v2"
	"github.com/demdxx/xtypes"
	"github.com/haxqer/vast"
	"go.uber.org/zap"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/adtype/prices"
	"github.com/geniusrabbit/adcorelib/billing"
	"github.com/geniusrabbit/adcorelib/context/ctxlogger"

	"github.com/geniusrabbit/adsource-openrtb/response/common"
)

// ResponseBidItem is the bid response item for VAST video ad formats.
// It implements [adtype.ResponseItem] and provides access to the decoded VAST
// document, media assets, trackers, and pricing information.
type ResponseBidItem struct {
	common.BaseBidItem

	// VAST-specific fields
	VAST   *vast.VAST     `json:"vast,omitempty"`
	Data   map[string]any `json:"data,omitempty"`
	assets admodels.AdFileAssets

	// Tracking links extracted from VAST document
	impressionTrackers []string
	clickTrackers      []string
	viewTrackers       []string
}

// Assets returns the file assets extracted from the VAST bid response.
func (it *ResponseBidItem) Assets() admodels.AdFileAssets { return it.assets }

// MainAsset returns the primary file asset matched against the format configuration.
func (it *ResponseBidItem) MainAsset() *admodels.AdFileAsset {
	return common.MainAssetOf(it.Format(), it.assets)
}

// New creates a ResponseBidItem for a VAST bid. It decodes and validates the
// XML VAST markup from the OpenRTB Bid, extracts tracking URLs and media assets.
func New(req adtype.BidRequester, src adtype.Source, bid *openrtb.Bid, imp *adtype.Impression, format *types.Format) (*ResponseBidItem, error) {
	cpmPrice := billing.MoneyFloat(bid.Price)
	bidItem := &ResponseBidItem{
		BaseBidItem: common.BaseBidItem{
			ItemID:     bid.ID,
			Src:        src,
			Req:        req,
			Imp:        imp,
			Bid:        bid,
			FormatType: types.FormatVideoType,
			RespFormat: format,
			PriceScope: prices.PriceScope{
				CPMScope: prices.CPMScope{MaxBidCPM: cpmPrice, BidCPM: cpmPrice},
				ECPM:     cpmPrice,
			},
		},
	}

	vastAd, err := unmarshalVAST([]byte(bid.AdMarkup))
	if err != nil {
		ctxlogger.Get(req.Context()).Debug(
			"Failed to decode VAST markup",
			zap.String("markup", bid.AdMarkup),
			zap.Error(err),
		)
		return nil, err
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
	var assets admodels.AdFileAssets
	if vastAd.Ads[0].InLine != nil {
		for _, creative := range vastAd.Ads[0].InLine.Creatives {
			if creative.Linear == nil || creative.Linear.MediaFiles == nil {
				continue
			}
			mediaAssets := fileAssetsFromMediaFiles(creative.Linear.MediaFiles.MediaFile,
				int(time.Duration(creative.Linear.Duration).Seconds()))
			assets = append(assets, mediaAssets...)
			if creative.Linear.Icons != nil && creative.Linear.Icons.Icon != nil {
				iconAssets := iconAssetsFromIcons(*creative.Linear.Icons.Icon)
				assets = append(assets, iconAssets...)
			}
		}
	} else if vastAd.Ads[0].Wrapper != nil {
		assets = admodels.AdFileAssets{
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
				assets = append(assets, iconAssets...)
			}
		}
	}

	// Cache the extracted assets in the bid item for future access.
	bidItem.assets = assets

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

// ActionURL returns the primary video click-through URL from the VAST document.
func (it *ResponseBidItem) ActionURL() string {
	if it.VAST.Ads[0].InLine != nil {
		return it.VAST.Ads[0].InLine.Creatives[0].Linear.VideoClicks.ClickThroughs[0].URI
	}
	return it.VAST.Ads[0].Wrapper.Creatives[0].Linear.VideoClicks.ClickThroughs[0].URI
}

// IsDirect always returns false for VAST bid items.
func (it *ResponseBidItem) IsDirect() bool {
	return false
}

// fileAssetsFromMediaFiles converts a slice of VAST [vast.MediaFile] descriptors
// into [admodels.AdFileAssets], inferring asset type from the MIME type prefix.
func fileAssetsFromMediaFiles(mediaFiles []vast.MediaFile, duration int) admodels.AdFileAssets {
	assets := make(admodels.AdFileAssets, 0, len(mediaFiles))
	for i, mf := range mediaFiles {
		assets = append(assets, &admodels.AdFileAsset{
			ID:          uint64(i + 1),
			URL:         mf.URI,
			Type:        assetTypeFromContentType(mf.Type, types.AdFileAssetUndefinedType),
			ContentType: mf.Type,
			Width:       int(mf.Width),
			Height:      int(mf.Height),
			Duration:    duration,
		})
	}
	return assets
}

// iconAssetsFromIcons converts a slice of VAST [vast.Icon] descriptors into
// [admodels.AdFileAssets] as icon-type image assets.
func iconAssetsFromIcons(icons []vast.Icon) admodels.AdFileAssets {
	assets := make(admodels.AdFileAssets, 0, len(icons))
	for i, icon := range icons {
		assets = append(assets, &admodels.AdFileAsset{
			ID:          uint64(1000 + i),
			URL:         icon.StaticResource.URI,
			Type:        assetTypeFromContentType(icon.StaticResource.CreativeType, types.AdFileAssetImageType),
			ContentType: icon.StaticResource.CreativeType,
			Width:       int(icon.Width),
			Height:      int(icon.Height),
		})
	}
	return assets
}

var _ adtype.ResponseItem = &ResponseBidItem{}
