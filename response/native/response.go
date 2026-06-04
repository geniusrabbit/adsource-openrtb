//
// @project GeniusRabbit corelib 2017 - 2019, 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019, 2025
//

// Package native implements the OpenRTB bid response item for native ad formats.
// Native ads deliver structured data (title, image, link, etc.) that is rendered
// by the publisher according to its own design guidelines.
package native

import (
	"errors"

	"github.com/bsm/openrtb"
	natresp "github.com/bsm/openrtb/native/response"
	"github.com/demdxx/gocast/v2"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/billing"
	"github.com/geniusrabbit/adcorelib/price"

	"github.com/geniusrabbit/adsource-openrtb/response/common"
)

// ErrMissingRequiredAsset is returned when a required native asset declared in
// the format config is absent from the bid response.
var ErrMissingRequiredAsset = errors.New("missing required native asset")

// ResponseBidItem is the bid response item for native ad formats.
// It implements [adtype.ResponseItem] and provides access to the decoded native
// assets, link, trackers, and pricing information.
type ResponseBidItem struct {
	common.BaseBidItem

	// Native-specific fields
	Native     *natresp.Response `json:"native,omitempty"`
	ActionLink string            `json:"action_link,omitempty"`
	Data       map[string]any    `json:"data,omitempty"`
	assets     admodels.AdFileAssets
}

// New creates a ResponseBidItem for a native bid. It decodes the JSON native markup
// from the OpenRTB Bid, maps asset IDs to content fields, and validates that all
// required format assets are present in the response.
func New(req adtype.BidRequester, src adtype.Source, bid *openrtb.Bid, imp *adtype.Impression, format *types.Format) (*ResponseBidItem, error) {
	native, err := decodeNativeMarkup([]byte(bid.AdMarkup))
	if err != nil {
		return nil, err
	}

	if err := validateRequiredAssets(format, native); err != nil {
		return nil, err
	}

	cpmPrice := billing.MoneyFloat(bid.Price)
	bidItem := &ResponseBidItem{
		BaseBidItem: common.BaseBidItem{
			ItemID:     imp.ID,
			Src:        src,
			Req:        req,
			Imp:        imp,
			Bid:        bid,
			FormatType: types.FormatNativeType,
			RespFormat: format,
			PriceScope: price.PriceScopeImpression{
				MaxBidImpPrice: 0,
				BidImpPrice:    0,
				ImpPrice:       cpmPrice / 1000, // Convert CPM to per-impression price
				ECPM:           cpmPrice,
			},
		},
		Native:     native,
		ActionLink: native.Link.URL,
		Data:       extractNativeDataFromImpression(imp, native),
	}

	// Ensure all required assets are present and extract media assets for future access.
	bidItem.PriceScope.MaxBidImpPrice =
		price.CalculatePurchasePrice(bidItem, adtype.ActionImpression)

	// Extract media assets and cache them in the bid item for future access.
	if format.Config != nil {
		for _, configAsset := range format.Config.Assets {
			for _, asset := range bidItem.Native.Assets {
				// Skip assets that don't match the config ID or carry no media.
				if asset.ID != configAsset.ID || (asset.Image == nil && asset.Video == nil) {
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
					newAsset.Width = asset.Image.Width
					newAsset.Height = asset.Image.Height
				case asset.Video != nil:
					newAsset.URL = asset.Video.VASTTag
					newAsset.Type = types.AdFileAssetVideoType
				}
				bidItem.assets = append(bidItem.assets, newAsset)
				break
			}
		}
	}

	return bidItem, nil
}

// Assets returns the file assets extracted from the native bid response.
func (it *ResponseBidItem) Assets() admodels.AdFileAssets { return it.assets }

// MainAsset returns the primary file asset matched against the format configuration.
func (it *ResponseBidItem) MainAsset() *admodels.AdFileAsset {
	return common.MainAssetOf(it.Format(), it.assets)
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

// ContentFields returns a map of all content fields for the native ad, keyed by
// the format field names defined in the format configuration.
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

// ImpressionTrackerLinks returns tracking links fired on impression.
func (it *ResponseBidItem) ImpressionTrackerLinks() []string {
	return it.Native.ImpTrackers
}

// ClickTrackerLinks returns third-party tracker URLs fired on click.
func (it *ResponseBidItem) ClickTrackerLinks() []string {
	return it.Native.Link.ClickTrackers
}

// ActionURL returns the main click-through URL from the native response link.
func (it *ResponseBidItem) ActionURL() string {
	return it.ActionLink
}

var _ adtype.ResponseItem = &ResponseBidItem{}
