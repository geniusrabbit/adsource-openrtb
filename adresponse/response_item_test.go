package adresponse

import (
	"reflect"
	"testing"

	"github.com/bsm/openrtb"
	"github.com/stretchr/testify/assert"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/billing"
	"github.com/geniusrabbit/adcorelib/price"
)

func TestItemPricing(t *testing.T) {
	var (
		acc = &admodels.Account{
			IDval:        1,
			RevenueShare: 0.9,
		}
		imp   = adtype.Impression{Target: &adtype.TargetEmpty{Acc: acc}}
		items = []adtype.ResponserItem{newRTBResponse(acc, imp)}
	)

	for _, item := range items {
		prefix := reflect.TypeOf(item).String()

		t.Run(prefix+"_empty_lead_price", func(t *testing.T) {
			assert.Equal(t, billing.MoneyFloat(0.), item.Price(adtype.ActionLead), "wrong_lead_price value")
		})

		t.Run(prefix+"_bid_price", func(t *testing.T) {
			assert.Equal(t, billing.MoneyFloat(10.), item.Price(adtype.ActionImpression), "wrong_bid_price value")
		})

		t.Run(prefix+"_commission_value", func(t *testing.T) {
			com := item.CommissionShareFactor() * item.Price(adtype.ActionImpression).Float64()
			assert.True(t, com >= 0.999 && com <= 1, "wrong_commission value")
		})
	}
}

func newRTBResponse(_ *admodels.Account, imp adtype.Impression) *ResponseBidItem {
	return &ResponseBidItem{
		ItemID:   "1",
		Src:      &adtype.SourceEmpty{PriceCorrectionReduce: 0},
		Req:      &adtype.BidRequest{ID: "xxx", Imps: []adtype.Impression{imp}},
		Imp:      &imp,
		Bid:      &openrtb.Bid{Price: 60},
		SecondAd: adtype.SecondAd{},
		PriceScope: price.PriceScopeView{
			MaxBidViewPrice: billing.MoneyFloat(10.),
			BidViewPrice:    billing.MoneyFloat(5.),
			ViewPrice:       billing.MoneyFloat(10.),
			ECPM:            billing.MoneyFloat(10.),
		},
	}
}
