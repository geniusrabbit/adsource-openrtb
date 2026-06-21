package rtbrules

import (
	"testing"

	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
)

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

var (
	benchFormat = &types.Format{Codename: "native"}
	benchImp    = &adtype.Impression{Push: true}
	benchTarget = &stubTarget{}

	benchRules = &RTBRules{
		Formats:             []string{"banner", "native", "video"},
		InterstitialFormats: []string{"proxy", "banner"},
		PushFormats:         []string{"native"},
		Rules: []*RuleItem{
			{
				Condition: Condition{
					Formats: []string{"proxy"},
					Push:    Exclude,
				},
				Config: RuleConfig{Ext: map[string]any{"format": 5}},
			},
			{
				Condition: Condition{
					Formats:      []string{"native"},
					Interstitial: Exclude,
					Push:         Include,
				},
				Config: RuleConfig{Ext: map[string]any{"type": "webpush"}},
				MapResponse: &MapResponse{
					Assets: []MapResponseAsset{
						{ID: 1, Name: "main", Field: "images/main/url"},
						{ID: 2, Name: "logo", Field: "images/icon/url"},
						{ID: 101, Name: "title", Field: "title"},
						{ID: 102, Name: "description", Field: "content"},
						{ID: 105, Name: "url", Field: "link/url"},
					},
				},
			},
		},
	}

	benchMapData = map[string]any{
		"title":   "Ad Title",
		"content": "Ad description text",
		"images": map[string]any{
			"main": map[string]any{"url": "https://cdn.example.com/main.jpg"},
			"icon": map[string]any{"url": "https://cdn.example.com/icon.png"},
		},
		"link": map[string]any{"url": "https://example.com/landing"},
	}

	benchMapResponse = &MapResponse{
		Assets: []MapResponseAsset{
			{ID: 1, Name: "main", Field: "images/main/url"},
			{ID: 2, Name: "logo", Field: "images/icon/url"},
			{ID: 101, Name: "title", Field: "title"},
			{ID: 102, Name: "description", Field: "content"},
			{ID: 105, Name: "url", Field: "link/url"},
			{ID: 106, Name: "missing", Field: "no/such/path"},
		},
	}
)

// ---------------------------------------------------------------------------
// Condition.Matches
// ---------------------------------------------------------------------------

func BenchmarkCondition_Matches_Hit(b *testing.B) {
	c := &Condition{Formats: []string{"native"}, Interstitial: Exclude, Push: Include}
	b.ReportAllocs()
	for b.Loop() {
		c.Matches(benchFormat, false, true)
	}
}

func BenchmarkCondition_Matches_Miss(b *testing.B) {
	c := &Condition{Formats: []string{"banner"}, Push: Include}
	b.ReportAllocs()
	for b.Loop() {
		c.Matches(benchFormat, false, true)
	}
}

// ---------------------------------------------------------------------------
// RTBRules format checks
// ---------------------------------------------------------------------------

func BenchmarkRTBRules_IsFormatSupport(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		benchRules.IsFormatSupport(benchFormat)
	}
}

func BenchmarkRTBRules_IsInterstitialSupport(b *testing.B) {
	f := &types.Format{Codename: "proxy"}
	b.ReportAllocs()
	for b.Loop() {
		benchRules.IsInterstitialSupport(f)
	}
}

func BenchmarkRTBRules_IsPushSupport(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		benchRules.IsPushSupport(benchFormat)
	}
}

// ---------------------------------------------------------------------------
// RTBRules.AdjustImpression
// ---------------------------------------------------------------------------

func BenchmarkRTBRules_AdjustImpression_Match(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = benchRules.AdjustImpression(benchTarget, benchImp, benchFormat)
	}
}

func BenchmarkRTBRules_AdjustImpression_NoMatch(b *testing.B) {
	noMatchImp := &adtype.Impression{Push: false}
	b.ReportAllocs()
	for b.Loop() {
		_ = benchRules.AdjustImpression(benchTarget, noMatchImp, benchFormat)
	}
}

// ---------------------------------------------------------------------------
// resolveFieldPath
// ---------------------------------------------------------------------------

func BenchmarkResolveFieldPath_Simple(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		resolveFieldPath(benchMapData, "title")
	}
}

func BenchmarkResolveFieldPath_Nested3(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		resolveFieldPath(benchMapData, "images/main/url")
	}
}

func BenchmarkResolveFieldPath_Missing(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		resolveFieldPath(benchMapData, "no/such/path")
	}
}

// ---------------------------------------------------------------------------
// MapResponse.Mapping
// ---------------------------------------------------------------------------

func BenchmarkMapResponse_Mapping_Cold(b *testing.B) {
	noop := func(_ string, _ any) error { return nil }
	b.ReportAllocs()
	for b.Loop() {
		// fresh MapResponse each iteration — segments not yet cached
		mr := &MapResponse{Assets: make([]MapResponseAsset, len(benchMapResponse.Assets))}
		copy(mr.Assets, benchMapResponse.Assets)
		_ = mr.Mapping(benchMapData, noop)
	}
}

func BenchmarkMapResponse_Mapping_Warm(b *testing.B) {
	noop := func(_ string, _ any) error { return nil }
	// prime the cache before the timed loop
	_ = benchMapResponse.Mapping(benchMapData, noop)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_ = benchMapResponse.Mapping(benchMapData, noop)
	}
}
