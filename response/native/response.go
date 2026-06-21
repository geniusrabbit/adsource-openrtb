//
// @project GeniusRabbit corelib 2017 - 2019, 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019, 2025
//

// Package native implements the OpenRTB bid response item for native ad formats.
// Native ads deliver structured data (title, image, link, etc.) that is rendered
// by the publisher according to its own design guidelines.
package native

import (
	"encoding/json"
	"errors"

	"github.com/bsm/openrtb"
	natresp "github.com/bsm/openrtb/native/response"
	"github.com/demdxx/gocast/v2"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/billing"
	"github.com/geniusrabbit/adcorelib/price"

	"github.com/geniusrabbit/adsource-openrtb/request/rtbrules"
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
func New(req adtype.BidRequester, src adtype.Source, bid *openrtb.Bid, imp *adtype.Impression, format *types.Format, rules rtbrules.RTBRuler) (*ResponseBidItem, error) {
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
		Native:     &natresp.Response{},
		ActionLink: "",
		Data:       nil,
	}

	// Apply mapping rules if provided. If no rules are applied, decode the native markup directly.
	appliedMappers := false
	if rules != nil && rules.HasMappingRules() {
		var data map[string]any
		if err := json.Unmarshal([]byte(bid.AdMarkup), &data); err != nil {
			return nil, err
		}
		err := rules.ApplyRules(format, imp.IsInterstitial(), imp.IsPush(),
			func(rule *rtbrules.RuleItem) error {
				if rule != nil && rule.MapResponse.HasAssets() {
					appliedMappers = true
					return rule.MapResponse.Mapping(data, bidItem.SetContentItem)
				}
				return nil
			})
		if err != nil {
			return nil, err
		}
	}

	// If no mapping rules were applied, decode the native markup directly from the bid.AdMarkup.
	if !appliedMappers {
		if err := decodeNativeMarkup(bidItem.Native, []byte(bid.AdMarkup)); err != nil {
			return nil, err
		}
		if err := validateRequiredAssets(format, bidItem.Native); err != nil {
			return nil, err
		}
		bidItem.Data = extractNativeDataFromImpression(imp, bidItem.Native)
	}

	// Set the main action link from the native response link.
	bidItem.ActionLink = bidItem.Native.Link.URL

	// Ensure all required assets are present and extract media assets for future access.
	bidItem.PriceScope.MaxBidImpPrice =
		price.CalculatePurchasePrice(bidItem, adtype.ActionImpression)

	// Extract media assets and cache them in the bid item for future access.
	if format.Config != nil && len(format.Config.Assets) > 0 && len(bidItem.Native.Assets) > 0 {
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
	if len(it.Data) > 0 {
		if val, ok := it.Data[name]; ok {
			return val
		}
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

// SetContentItem sets the value of a named content field in the bid response.
// It updates the native response structure and the Data map, and also adds any
// associated file assets to the assets slice.
func (it *ResponseBidItem) SetContentItem(name string, value any) error {
	switch name {
	case adtype.ContentItemLink:
		it.Native.Link.URL = gocast.Str(value)
	case adtype.ContentItemNotifyWinURL:
		if it.Bid != nil {
			it.Bid.NURL = gocast.Str(value)
		}
	case adtype.ContentItemNotifyDisplayURL:
		if it.Bid != nil {
			it.Bid.BURL = gocast.Str(value)
		}
	default:
		if it.RespFormat != nil && it.RespFormat.Config != nil {
			if asset := it.RespFormat.Config.AssetByName(name); asset != nil {
				fileURL := gocast.Str(value)
				fileAsset := &admodels.AdFileAsset{
					ID:          uint64(asset.ID),
					Name:        name,
					ContentType: extractContentTypeFromFileName(fileURL),
					URL:         fileURL,
				}
				switch {
				case asset.IsImageSupport():
					fileAsset.Type = types.AdFileAssetImageType
				case asset.IsVideoSupport():
					fileAsset.Type = types.AdFileAssetVideoType
				}
				it.assets = append(it.assets, fileAsset)
				return nil
			}
		}
		if it.Data == nil {
			it.Data = make(map[string]any)
		}
		it.Data[name] = value
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
