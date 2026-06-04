//
// @project GeniusRabbit corelib 2017 - 2019, 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019, 2025
//

// Package common provides [BaseBidItem] — the shared struct and method set that is
// common to every per-format RTB bid response item (banner, direct, native, vast).
// Concrete response types embed [BaseBidItem] by value and implement only the
// format-specific methods (ContentItem, trackers, asset extraction, etc.).
package common

import (
	"context"
	"encoding/base64"

	"github.com/bsm/openrtb"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/billing"
	"github.com/geniusrabbit/adcorelib/price"
)

// BaseBidItem contains all fields and method implementations that are shared by
// every format-specific RTB bid response item. Embed this struct by value:
//
//	type ResponseBidItem struct {
//	    common.BaseBidItem
//	    // format-specific fields …
//	}
type BaseBidItem struct {
	ItemID string `json:"id"`

	// Request and impression data
	Src adtype.Source       `json:"source,omitempty"`
	Req adtype.BidRequester `json:"request,omitempty"`
	Imp *adtype.Impression  `json:"impression,omitempty"`

	// Format of response advertisement item
	FormatType types.FormatType `json:"format_type,omitempty"`
	RespFormat *types.Format    `json:"format,omitempty"`

	// External response data from RTB source
	Bid *openrtb.Bid `json:"bid,omitempty"`

	PriceScope price.PriceScopeImpression `json:"price_scope,omitempty"`

	// Competitive second AD
	SecondAd adtype.SecondAd `json:"second_ad,omitempty"`

	ctx context.Context
}

///////////////////////////////////////////////////////////////////////////////
// Identity
///////////////////////////////////////////////////////////////////////////////

// ID returns the unique identifier of the response item.
func (it *BaseBidItem) ID() string {
	return it.ItemID
}

// Source returns the ad source associated with this bid.
func (it *BaseBidItem) Source() adtype.Source {
	return it.Src
}

// NetworkName returns the network name for this source (always empty for RTB).
func (it *BaseBidItem) NetworkName() string {
	return ""
}

// AdID returns the system advertisement ID (always empty for RTB).
func (it *BaseBidItem) AdID() string {
	return ""
}

// CampaignID returns the campaign ID (always 0 for RTB sources).
func (it *BaseBidItem) CampaignID() uint64 {
	return 0
}

// AccountID returns the account ID from the ad source.
func (it *BaseBidItem) AccountID() uint64 {
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

// CreativeID returns the creative ID from the RTB bid.
func (it *BaseBidItem) CreativeID() string {
	if it == nil || it.Bid == nil {
		return ""
	}
	return it.Bid.CreativeID
}

///////////////////////////////////////////////////////////////////////////////
// Impression
///////////////////////////////////////////////////////////////////////////////

// Impression returns the impression object associated with this bid item.
func (it *BaseBidItem) Impression() *adtype.Impression {
	return it.Imp
}

// ImpressionID returns the unique impression identifier.
func (it *BaseBidItem) ImpressionID() string {
	if it.Imp == nil {
		return ""
	}
	return it.Imp.ID
}

// ExtImpressionID returns the external (RTB) impression identifier.
func (it *BaseBidItem) ExtImpressionID() string {
	if it.Imp == nil {
		return ""
	}
	return it.Imp.ExternalID
}

// ExtTargetID returns the external target identifier.
func (it *BaseBidItem) ExtTargetID() string {
	return it.Imp.ExternalTargetID
}

// TargetCodename returns the codename of the target placement.
func (it *BaseBidItem) TargetCodename() string {
	return it.Imp.TargetCodename()
}

///////////////////////////////////////////////////////////////////////////////
// Format
///////////////////////////////////////////////////////////////////////////////

// Format returns the matched format object for this bid item.
func (it *BaseBidItem) Format() *types.Format {
	if it == nil {
		return nil
	}
	return it.RespFormat
}

// PriorityFormatType returns the primary format type. Concrete types may override
// this when the format is always fixed (e.g. direct always returns FormatDirectType).
func (it *BaseBidItem) PriorityFormatType() types.FormatType {
	if it.FormatType != types.FormatUndefinedType {
		return it.FormatType
	}
	format := it.Imp.FormatTypes
	if formatType := format.HasOneType(); formatType > types.FormatUndefinedType {
		return formatType
	}
	return format.FirstType()
}

///////////////////////////////////////////////////////////////////////////////
// Assets
///////////////////////////////////////////////////////////////////////////////

// Assets returns nil — banner and direct bid items carry no file assets.
// Concrete types that do have assets (native, vast) override this method.
func (it *BaseBidItem) Assets() admodels.AdFileAssets { return nil }

// MainAsset returns nil for types that carry no file assets.
// Concrete types that do have assets (native, vast) override this method.
func (it *BaseBidItem) MainAsset() *admodels.AdFileAsset { return nil }

// MainAssetOf returns the primary file asset matched against the format configuration.
// Call this from concrete types that maintain their own assets slice.
func MainAssetOf(format *types.Format, assets admodels.AdFileAssets) *admodels.AdFileAsset {
	if format == nil || format.Config == nil {
		return nil
	}
	mainAsset := format.Config.MainAsset()
	if mainAsset == nil {
		return nil
	}
	for _, asset := range assets {
		if int(asset.ID) == mainAsset.ID {
			return asset
		}
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// Tracker defaults (nil — overridden by format-specific types that provide them)
///////////////////////////////////////////////////////////////////////////////

// ImpressionTrackerLinks returns tracking links fired on impression.
// Returns nil by default; overridden by banner, native, and vast.
func (it *BaseBidItem) ImpressionTrackerLinks() []string { return nil }

// ViewTrackerLinks returns tracking links fired on a viewable impression.
// Returns nil by default; overridden by banner and vast.
func (it *BaseBidItem) ViewTrackerLinks() []string { return nil }

// ClickTrackerLinks returns third-party tracker URLs fired on click.
// Returns nil by default; overridden by banner, native, and vast.
func (it *BaseBidItem) ClickTrackerLinks() []string { return nil }

///////////////////////////////////////////////////////////////////////////////
// Pricing
///////////////////////////////////////////////////////////////////////////////

// PricingModel returns CPM as the pricing model for all RTB bid items.
func (it *BaseBidItem) PricingModel() types.PricingModel {
	return types.PricingModelCPM
}

// FixedPurchasePrice returns the fixed purchase price for the given action from the impression.
func (it *BaseBidItem) FixedPurchasePrice(action adtype.Action) billing.Money {
	return it.Imp.PurchasePrice(action)
}

// ECPM returns the effective cost per mille for this bid.
func (it *BaseBidItem) ECPM() billing.Money {
	if it == nil || it.Bid == nil {
		return 0
	}
	return it.PriceScope.ECPM
}

// PriceTestMode always returns false for RTB bid items.
func (it *BaseBidItem) PriceTestMode() bool { return false }

// Price returns the total price for the given action (impression, click, lead, view).
func (it *BaseBidItem) Price(action adtype.Action) billing.Money {
	if it == nil || it.Bid == nil {
		return 0
	}
	return it.PriceScope.PricePerAction(action)
}

// BidImpressionPrice returns the bid price that the system will pay for an impression.
func (it *BaseBidItem) BidImpressionPrice() billing.Money {
	return it.PriceScope.BidImpPrice
}

// SetBidImpressionPrice sets the bid impression price. Returns an error if the new
// price exceeds the maximum allowed bid price.
func (it *BaseBidItem) SetBidImpressionPrice(bid billing.Money) error {
	if !it.PriceScope.SetBidImpressionPrice(bid, false) {
		return adtype.ErrNewAuctionBidIsHigherThenMaxBid
	}
	return nil
}

// PrepareBidImpressionPrice adjusts the given price according to source correction
// and commission factors.
func (it *BaseBidItem) PrepareBidImpressionPrice(p billing.Money) billing.Money {
	return it.PriceScope.PrepareBidImpressionPrice(p)
}

// InternalAuctionCPMBid returns the maximal possible price without any commission.
func (it *BaseBidItem) InternalAuctionCPMBid() billing.Money {
	return price.CalculateInternalAuctionBid(it)
}

// PurchasePrice returns the actual cost of the given action for the system.
func (it *BaseBidItem) PurchasePrice(action adtype.Action) billing.Money {
	return price.CalculatePurchasePrice(it, action)
}

// PotentialPrice returns the price that could have been received but was marked as discrepancy.
func (it *BaseBidItem) PotentialPrice(action adtype.Action) billing.Money {
	return price.CalculatePotentialPrice(it, action)
}

// FinalPrice returns the price after all corrections and commissions for the given action.
func (it *BaseBidItem) FinalPrice(action adtype.Action) billing.Money {
	return price.CalculateFinalPrice(it, action)
}

// Second returns the competitive second ad slot.
func (it *BaseBidItem) Second() *adtype.SecondAd {
	return &it.SecondAd
}

///////////////////////////////////////////////////////////////////////////////
// Revenue share / commission
///////////////////////////////////////////////////////////////////////////////

// CommissionShareFactor returns the commission fraction (0..1) taken from the publisher.
func (it *BaseBidItem) CommissionShareFactor() float64 {
	return it.Imp.CommissionShareFactor()
}

// SourceCorrectionFactor returns the price correction factor for this RTB source.
func (it *BaseBidItem) SourceCorrectionFactor() float64 {
	return it.Src.PriceCorrectionReduceFactor()
}

// TargetCorrectionFactor returns the revenue-share reduction factor for the target.
func (it *BaseBidItem) TargetCorrectionFactor() float64 {
	return it.Imp.Target.RevenueShareReduceFactor()
}

///////////////////////////////////////////////////////////////////////////////
// Creative metadata
///////////////////////////////////////////////////////////////////////////////

// RTBCategories returns the IAB content categories declared in the bid.
func (it *BaseBidItem) RTBCategories() []string {
	if it.Bid == nil {
		return nil
	}
	return it.Bid.Cat
}

// IsBackup always returns false for RTB bid items.
func (it *BaseBidItem) IsBackup() bool { return false }

// IsDirect reports whether the impression is a direct ad format.
// Overridden by direct (always true) and vast (always false).
func (it *BaseBidItem) IsDirect() bool {
	return it.Imp.IsDirect()
}

// Width returns the creative width in pixels.
func (it *BaseBidItem) Width() int {
	if it.Bid == nil {
		return 0
	}
	return it.Bid.W
}

// Height returns the creative height in pixels.
func (it *BaseBidItem) Height() int {
	if it.Bid == nil {
		return 0
	}
	return it.Bid.H
}

// Markup returns the rendered ad markup string. Returns empty string by default;
// format-specific types may override when they pre-render markup.
func (it *BaseBidItem) Markup() (string, error) {
	return "", nil
}

// Validate checks that all required fields are populated.
func (it *BaseBidItem) Validate() error {
	if it.Src == nil || it.Req == nil || it.Imp == nil || it.Bid == nil {
		return adtype.ErrInvalidItemInitialisation
	}
	return it.Bid.Validate()
}

///////////////////////////////////////////////////////////////////////////////
// Context
///////////////////////////////////////////////////////////////////////////////

// Context gets or sets the context associated with this bid item.
func (it *BaseBidItem) Context(ctx ...context.Context) context.Context {
	if len(ctx) > 0 {
		it.ctx = ctx[0]
	}
	return it.ctx
}

// Get retrieves a value from the item context by key.
func (it *BaseBidItem) Get(key string) (res any) {
	if it.ctx == nil {
		return res
	}
	return it.ctx.Value(key)
}

// ContentMapping returns a map that can be used to prepare ad markup content
func (it *BaseBidItem) ContentMapping() map[string]string {
	cpmPrice := (it.FinalPrice(adtype.ActionImpression) * 1000).String()
	base64Price := base64.URLEncoding.EncodeToString([]byte(cpmPrice))
	return map[string]string{
		"${AUCTION_PRICE}":             cpmPrice,
		"${AUCTION_PRICE:B64}":         base64Price,
		"%24%7BAUCTION_PRICE%7D":       cpmPrice,
		"%24%7BAUCTION_PRICE%3AB64%7D": base64Price,
	}
}
