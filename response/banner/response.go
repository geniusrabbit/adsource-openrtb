//
// @project GeniusRabbit corelib 2017 - 2019, 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019, 2025
//

// Package banner implements the OpenRTB bid response item for banner ad formats.
// It handles HTML, iframe, and image banner creatives received from RTB sources,
// including interstitial XML markup parsing.
package banner

import (
	"errors"
	"strings"

	"github.com/bsm/openrtb"
	"github.com/demdxx/gocast/v2"

	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/adtype/prices"
	"github.com/geniusrabbit/adcorelib/billing"

	"github.com/geniusrabbit/adsource-openrtb/response/common"
	"github.com/geniusrabbit/adsource-openrtb/response/internal/interstitial"
)

// ErrInvalidAdContent is returned when the ad markup cannot be parsed into valid banner content.
var ErrInvalidAdContent = errors.New("invalid ad content")

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
	common.BaseBidItem
	BannerInfo BannerInfo `json:"banner_info"`
}

// New creates a ResponseBidItem for a banner bid. It parses the ad markup from
// the OpenRTB Bid and returns [ErrInvalidAdContent] if the markup cannot be
// converted into renderable content.
func New(req adtype.BidRequester, src adtype.Source, bid *openrtb.Bid, imp *adtype.Impression, format *types.Format) (*ResponseBidItem, error) {
	cpmPrice := billing.MoneyFloat(bid.Price)
	bidItem := &ResponseBidItem{
		BaseBidItem: common.BaseBidItem{
			ItemID:     imp.ID,
			Src:        src,
			Req:        req,
			Imp:        imp,
			Bid:        bid,
			FormatType: bannerFormatType(bid.AdMarkup),
			RespFormat: format,
			PriceScope: prices.PriceScope{
				CPMScope: prices.CPMScope{MaxBidCPM: cpmPrice, BidCPM: cpmPrice},
				ECPM:     cpmPrice,
			},
		},
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
	case adtype.ContentItemLink:
		return it.BannerInfo.LinkURL
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

// ActionURL returns the click-through URL for the banner ad.
func (it *ResponseBidItem) ActionURL() string {
	return it.BannerInfo.LinkURL
}

var _ adtype.ResponseItem = &ResponseBidItem{}
