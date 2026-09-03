package rules

import (
	"slices"
	"testing"

	"github.com/geniusrabbit/adsource-openrtb/request/rtbrules"
)

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
	direct := r.Detect(rtbrules.RequestSignal{Ext: map[string]any{"format": float64(4)}})
	if direct == nil || !slices.Equal(direct.FormatCodes, []string{"direct"}) {
		t.Fatalf("format=4: %+v", direct)
	}

	intr := r.Detect(rtbrules.RequestSignal{Ext: map[string]any{"format": 5}})
	if intr == nil || !intr.Interstitial || !slices.Equal(intr.FormatCodes, []string{"proxy"}) {
		t.Fatalf("format=5: %+v", intr)
	}

	banner := r.Detect(rtbrules.RequestSignal{Ext: map[string]any{"format": 1}})
	if banner == nil {
		t.Fatal("format=1: no match")
	}
	want := []string{"proxy_300x250", "proxy_300x100", "proxy_728x90", "banner_300x250", "banner_300x100", "banner_728x90"}
	if !slices.Equal(banner.FormatCodes, want) {
		t.Errorf("format=1 codes=%v want %v", banner.FormatCodes, want)
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
