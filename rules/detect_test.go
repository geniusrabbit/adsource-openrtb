package rules

import (
	"slices"
	"testing"

	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adsource-openrtb/request/rtbrules"
)

type extTarget struct{ ext map[string]any }

func (s *extTarget) SetExt(ext map[string]any) { s.ext = ext }

func TestByName(t *testing.T) {
	if ByName("twinred") != Rules["twinred"] {
		t.Fatal("twinred should resolve")
	}
	if ByName("unknown") != Rules["default"] {
		t.Fatal("unknown name should fall back to default")
	}
	if ByName("") != Rules["default"] {
		t.Fatal("empty name should fall back to default")
	}
}

func TestDetect_TwinRed(t *testing.T) {
	r := Rules["twinred"]
	for _, ext := range []map[string]any{
		{"format": float64(4)},
		{"format": float64(1)},
		{"format": float64(5)},
		nil,
	} {
		got := r.Detect(rtbrules.RequestSignal{Ext: ext})
		if got == nil || got.Interstitial || !slices.Equal(got.FormatCodes, []string{"direct"}) {
			t.Fatalf("ext=%v: %+v", ext, got)
		}
	}
	if got := r.Detect(rtbrules.RequestSignal{Instl: true, Ext: map[string]any{"format": float64(5)}}); got != nil {
		t.Fatalf("interstitial: %+v", got)
	}
}

func TestAdjustImpression_TwinRed(t *testing.T) {
	r := Rules["twinred"]
	target := &extTarget{}
	format := &types.Format{Codename: "direct"}
	imp := &adtype.Impression{}
	if err := r.AdjustImpression(target, imp, format); err != nil {
		t.Fatal(err)
	}
	if target.ext["format"] != 4 {
		t.Fatalf("direct ext=%v", target.ext)
	}
}

func TestDetect_TrafficStars(t *testing.T) {
	r := Rules["trafficstars"]
	push := r.Detect(rtbrules.RequestSignal{Ext: map[string]any{"type": "webpush"}})
	if push == nil || !push.Push || !push.NoRequestObject || !slices.Equal(push.FormatCodes, []string{"native"}) {
		t.Fatalf("webpush: %+v", push)
	}
	if push.Rule == nil || !push.Rule.MapResponse.HasAssets() {
		t.Fatal("webpush rule should carry MapResponse")
	}

	pop := r.Detect(rtbrules.RequestSignal{Ext: map[string]any{"type": "pop"}})
	if pop == nil || !slices.Equal(pop.FormatCodes, []string{"direct"}) {
		t.Fatalf("pop: %+v", pop)
	}
}

func TestDetect_DefaultPop(t *testing.T) {
	r := Rules["default"]
	got := r.Detect(rtbrules.RequestSignal{Ext: map[string]any{"type": "pop"}})
	if got == nil || !slices.Equal(got.FormatCodes, []string{"direct"}) {
		t.Fatalf("default pop: %+v", got)
	}
	if got := r.Detect(rtbrules.RequestSignal{HasBanner: true, Ext: map[string]any{"foo": 1}}); got != nil {
		t.Fatalf("banner without matching ext should be nil, got %+v", got)
	}
}
