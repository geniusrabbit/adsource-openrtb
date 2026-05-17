//
// @project GeniusRabbit corelib 2017 - 2019, 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019, 2025
//

// Package native implements the OpenRTB bid response item for native ad formats.
// Native ads deliver structured data (title, image, link, etc.) that is rendered
// by the publisher according to its own design guidelines.
package native

import (
	"context"
	"errors"
	"fmt"

	"github.com/bsm/openrtb"
	natresp "github.com/bsm/openrtb/native/response"
	"github.com/demdxx/gocast/v2"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/billing"
	"github.com/geniusrabbit/adcorelib/price"
)

// ErrMissingRequiredAsset is returned when a required native asset declared in
// the format config is absent from the bid response.
var ErrMissingRequiredAsset = errors.New("missing required native asset")

// ResponseBidItem is the bid response item for native ad formats.
// It implements [adtype.ResponseItem] and provides access to the decoded native
// assets, link, trackers, and pricing information.
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
		FormatType: types.FormatNativeType,
		RespFormat: format,
		Native:     native,
		ActionLink: native.Link.URL,
		Data:       extractNativeDataFromImpression(imp, native),
		PriceScope: priceScope,
	}

	bidItem.PriceScope.MaxBidImpPrice = price.CalculatePurchasePrice(bidItem, adtype.ActionImpression)
	return bidItem, nil
}

// validateRequiredAssets checks that every required image/video asset declared in
// the format config is present (by matching asset ID) in the native response.
// Returns [ErrMissingRequiredAsset] on the first missing required asset.
func validateRequiredAssets(format *types.Format, native *natresp.Response) error {
	if format == nil || format.Config == nil {
		return nil
	}
	for _, configAsset := range format.Config.Assets {
		if !configAsset.IsRequired() {
			continue
		}
		found := false
		for _, asset := range native.Assets {
			if asset.ID == configAsset.ID && (asset.Image != nil || asset.Video != nil) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: asset id=%d name=%q", ErrMissingRequiredAsset, configAsset.ID, configAsset.GetName())
		}
	}
	return nil
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

// ViewTrackerLinks returns tracking links fired on viewable impression (nil for native).
func (it *ResponseBidItem) ViewTrackerLinks() []string {
	return nil
}

// ClickTrackerLinks returns third-party tracker URLs fired on click.
func (it *ResponseBidItem) ClickTrackerLinks() []string {
	return it.Native.Link.ClickTrackers
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
// On the first call the list is built lazily by matching format config asset IDs
// against the image/video assets present in the native response.
func (it *ResponseBidItem) Assets() admodels.AdFileAssets {
	if it.assets != nil || it.Format().Config == nil {
		return it.assets
	}
	config := it.Format().Config
	for _, configAsset := range config.Assets {
		for _, asset := range it.Native.Assets {
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
			it.assets = append(it.assets, newAsset)
			break
		}
	}
	return it.assets
}

// Format returns the matched format object for this bid item.
func (it *ResponseBidItem) Format() *types.Format {
	if it == nil {
		return nil
	}
	return it.RespFormat
}

// PriorityFormatType returns FormatNativeType for all native bid items.
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

// PricingModel returns CPM as the pricing model for all RTB native bids.
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

// PriceTestMode always returns false for RTB native bids.
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

// IsDirect reports whether this is a direct ad format.
func (it *ResponseBidItem) IsDirect() bool {
	return it.Imp.IsDirect()
}

// IsBackup always returns false for native bid items.
func (it *ResponseBidItem) IsBackup() bool { return false }

// ActionURL returns the main click-through URL from the native response link.
func (it *ResponseBidItem) ActionURL() string {
	return it.ActionLink
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

// Markup returns the rendered ad markup (empty for native; rendered by template engine).
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
