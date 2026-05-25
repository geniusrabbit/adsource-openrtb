package v2

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adquery/bidrequest"
	"github.com/geniusrabbit/adcorelib/adtype"

	requestoptions "github.com/geniusrabbit/adsource-openrtb/request/options"
)

func TestRequestToRTBv2_ID(t *testing.T) {
	req := makeBidRequest()
	b := New(nil)
	result, _ := b.requestToRTBv2(req)
	assert.Equal(t, req.ID(), result.ID)
}

func TestRequestToRTBv2_HasImpressions(t *testing.T) {
	req := makeBidRequest()
	b := New(nil)
	result, _ := b.requestToRTBv2(req)
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
	b := New(nil)
	result, _ := b.requestToRTBv2(req)

	require.NotEmpty(t, result.Imp)
	require.NotNil(t, result.Imp[0].Banner)
}

func TestRequestToRTBv2_DefaultCurrencyUSD(t *testing.T) {
	req := makeBidRequest()
	b := New(nil)
	result, _ := b.requestToRTBv2(req)
	assert.Contains(t, result.Cur, "USD")
}

func TestRequestToRTBv2_WithCustomCurrency(t *testing.T) {
	req := makeBidRequest()
	b := New(nil)
	result, _ := b.requestToRTBv2(req, withCurrencies("EUR", "GBP"))
	assert.Equal(t, []string{"EUR", "GBP"}, result.Cur)
}

func TestRequestToRTBv2_AuctionType(t *testing.T) {
	req := makeBidRequest()
	b := New(nil)
	result, _ := b.requestToRTBv2(req, requestoptions.WithAuctionType(types.AuctionType(2)))
	assert.Equal(t, 2, result.AuctionType)
}

func TestRequestToRTBv2_TMax(t *testing.T) {
	req := makeBidRequest()
	b := New(nil)
	result, _ := b.requestToRTBv2(req, requestoptions.WithMaxTimeDuration(200*time.Millisecond))
	assert.Equal(t, 200, result.TMax)
}

func TestRequestToRTBv2_BidFloor(t *testing.T) {
	imp := makeBannerImpression()
	imp.BidFloorCPM = 0
	req := makeBidRequestWithImps([]*adtype.Impression{imp})
	b := New(nil)
	result, _ := b.requestToRTBv2(req, requestoptions.WithBidFloor(0.5))

	require.NotEmpty(t, result.Imp)
	assert.GreaterOrEqual(t, result.Imp[0].BidFloor, 0.5)
}

func TestRequestToRTBv2_NoImpressions(t *testing.T) {
	req := makeBidRequestWithImps([]*adtype.Impression{})
	b := New(nil)
	result, _ := b.requestToRTBv2(req)
	assert.Nil(t, result)
}

func TestBuilder_Build(t *testing.T) {
	b := New(nil)
	req := makeBidRequest()
	result, err := b.Build(req)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NoError(t, result.Validate())
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func withCurrencies(currencies ...string) requestoptions.BidRequestRTBOption {
	return func(opts *requestoptions.BidRequestRTBOptions) {
		opts.Currency = currencies
	}
}

func makeBidRequest() *bidrequest.BidRequest {
	return &bidrequest.BidRequest{
		IDVal: "test-bid-id",
		Ctx:   context.Background(),
		Imps:  []*adtype.Impression{makeBannerImpression()},
	}
}

func makeBidRequestWithImps(imps []*adtype.Impression) *bidrequest.BidRequest {
	base := makeBidRequest()
	base.Imps = imps
	return base
}

func makeBannerImpression() *adtype.Impression {
	imp := &adtype.Impression{
		ID:     "imp-1",
		Width:  300,
		Height: 250,
		Target: &adtype.TargetEmpty{},
	}
	formats := types.MockFormats()
	var bannerFormats []*types.Format
	for _, f := range formats {
		if f.IsBanner() {
			bannerFormats = append(bannerFormats, f)
			break
		}
	}
	if len(bannerFormats) > 0 {
		imp.InitFormats(types.NewSimpleFormatAccessor(bannerFormats))
	}
	return imp
}
