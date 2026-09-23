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
	wantBanner := []string{
		"proxy_300x250", "proxy_300x100", "proxy_728x90",
		"banner_300x250", "banner_300x100", "banner_728x90",
	}
	direct := r.Detect(rtbrules.RequestSignal{})
	if direct == nil || direct.Interstitial || !slices.Equal(direct.FormatCodes, []string{"direct"}) {
		t.Fatalf("popunder: %+v", direct)
	}
	if direct.Rule == nil || direct.Rule.Response == nil || direct.Rule.Response.Render != "rawURL" {
		t.Fatalf("popunder render: %+v", direct.Rule)
	}
	banner := r.Detect(rtbrules.RequestSignal{HasBanner: true, Ext: map[string]any{"format": float64(1)}})
	if banner == nil || !slices.Equal(banner.FormatCodes, wantBanner) {
		t.Fatalf("banner: %+v", banner)
	}
	video := r.Detect(rtbrules.RequestSignal{HasVideo: true})
	if video == nil || !slices.Equal(video.FormatCodes, []string{"video"}) {
		t.Fatalf("video: %+v", video)
	}
	instl := r.Detect(rtbrules.RequestSignal{Instl: true, Ext: map[string]any{"format": float64(5)}})
	if instl == nil || !instl.Interstitial || !slices.Equal(instl.FormatCodes, []string{"proxy"}) {
		t.Fatalf("interstitial: %+v", instl)
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
