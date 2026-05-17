package adsourceopenrtb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adquery/bidrequest"
	"github.com/geniusrabbit/adcorelib/adtype"
)

// ─── RTB v2 conversion ───────────────────────────────────────────────────────

func TestRequestToRTBv2_ID(t *testing.T) {
	req := makeBidRequest()
	result := requestToRTBv2(req)
	assert.Equal(t, req.ID(), result.ID)
}

func TestRequestToRTBv2_HasImpressions(t *testing.T) {
	req := makeBidRequest()
	result := requestToRTBv2(req)
	assert.NotEmpty(t, result.Imp, "expected at least one OpenRTB impression")
}

func TestRequestToRTBv2_BannerImpressionSize(t *testing.T) {
	imp := &adtype.Impression{
		ID:     "imp-size",
		Width:  300,
		Height: 300,
		Target: &adtype.TargetEmpty{},
	}
	formats := types.MockFormats()
	var bannerFormats []*types.Format
	for _, f := range formats {
		if f.IsBanner() {
			bannerFormats = append(bannerFormats, f)
		}
	}
	require.NotEmpty(t, bannerFormats)
	imp.InitFormats(types.NewSimpleFormatAccessor(bannerFormats))
	require.NotEmpty(t, imp.Formats(), "impression should have at least one format after InitFormats")

	req := makeBidRequestWithImps([]*adtype.Impression{imp})
	result := requestToRTBv2(req)

	require.NotEmpty(t, result.Imp)
	require.NotNil(t, result.Imp[0].Banner)
}

func TestRequestToRTBv2_DefaultCurrencyUSD(t *testing.T) {
	req := makeBidRequest()
	result := requestToRTBv2(req)
	assert.Contains(t, result.Cur, "USD")
}

func TestRequestToRTBv2_WithCustomCurrency(t *testing.T) {
	req := makeBidRequest()
	result := requestToRTBv2(req, WithCurrencies("EUR", "GBP"))
	assert.Equal(t, []string{"EUR", "GBP"}, result.Cur)
}

func TestRequestToRTBv2_AuctionType(t *testing.T) {
	req := makeBidRequest()
	result := requestToRTBv2(req, WithAuctionType(types.AuctionType(2)))
	assert.Equal(t, 2, result.AuctionType)
}

func TestRequestToRTBv2_TMax(t *testing.T) {
	req := makeBidRequest()
	result := requestToRTBv2(req, WithMaxTimeDuration(200*time.Millisecond))
	assert.Equal(t, 200, result.TMax)
}

func TestRequestToRTBv2_BidFloor(t *testing.T) {
	imp := makeBannerImpression()
	imp.BidFloorCPM = 0 // ensure we test the option path
	req := makeBidRequestWithImps([]*adtype.Impression{imp})
	result := requestToRTBv2(req, WithBidFloor(0.5))

	require.NotEmpty(t, result.Imp)
	assert.GreaterOrEqual(t, result.Imp[0].BidFloor, 0.5)
}

func TestRequestToRTBv2_NoImpressions(t *testing.T) {
	// Without impressions the request will have empty Imp slice
	req := makeBidRequestWithImps([]*adtype.Impression{})
	result := requestToRTBv2(req)
	assert.Empty(t, result.Imp)
}

// ─── RTB v3 conversion ───────────────────────────────────────────────────────

func TestRequestToRTBv3_ID(t *testing.T) {
	req := makeBidRequest()
	result := requestToRTBv3(req)
	assert.Equal(t, req.ID(), result.ID)
}

func TestRequestToRTBv3_HasImpressions(t *testing.T) {
	req := makeBidRequest()
	result := requestToRTBv3(req)
	assert.NotEmpty(t, result.Impressions, "expected at least one OpenRTB v3 impression")
}

func TestRequestToRTBv3_DefaultCurrencyUSD(t *testing.T) {
	req := makeBidRequest()
	result := requestToRTBv3(req)
	assert.Contains(t, result.Currencies, "USD")
}

func TestRequestToRTBv3_AuctionType(t *testing.T) {
	req := makeBidRequest()
	result := requestToRTBv3(req, WithAuctionType(types.AuctionType(1)))
	assert.Equal(t, 1, result.AuctionType)
}

func TestRequestToRTBv3_TMax(t *testing.T) {
	req := makeBidRequest()
	result := requestToRTBv3(req, WithMaxTimeDuration(100*time.Millisecond))
	assert.Equal(t, 100, result.TimeMax)
}

// ─── BidRequestRTBOptions ────────────────────────────────────────────────────

func TestWithBidFloor_Negative(t *testing.T) {
	var opts BidRequestRTBOptions
	WithBidFloor(-5.0)(&opts)
	assert.Equal(t, 0.0, opts.BidFloor, "negative bid floor should be clamped to 0")
}

func TestWithFormatFilter(t *testing.T) {
	called := false
	filterFn := func(f *types.Format) bool {
		called = true
		return true
	}
	var opts BidRequestRTBOptions
	WithFormatFilter(filterFn)(&opts)
	opts.FormatFilter(&types.Format{})
	assert.True(t, called, "FormatFilter should have been called")
}

func TestWithRTBOpenNativeVersion(t *testing.T) {
	var opts BidRequestRTBOptions
	WithRTBOpenNativeVersion("1.2")(&opts)
	assert.Equal(t, "1.2", opts.openNativeVer())
}

func TestBidRequestRTBOptions_DefaultCurrency(t *testing.T) {
	var opts BidRequestRTBOptions
	assert.Equal(t, []string{"USD"}, opts.currencies())
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// WithCurrencies is a test helper option.
func WithCurrencies(currencies ...string) BidRequestRTBOption {
	return func(opts *BidRequestRTBOptions) {
		opts.Currency = currencies
	}
}

func makeBidRequestWithImps(imps []*adtype.Impression) *bidrequest.BidRequest {
	base := makeBidRequest()
	base.Imps = imps
	return base
}
