package rtbrules

import (
	"errors"
	"testing"

	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func fmt(codename string) *types.Format { return &types.Format{Codename: codename} }

func imp(intr, push bool) *adtype.Impression {
	return &adtype.Impression{Interstitial: intr, Push: push}
}

// stubTarget captures the last SetExt call.
type stubTarget struct{ ext map[string]any }

func (s *stubTarget) SetExt(ext map[string]any) { s.ext = ext }

// ---------------------------------------------------------------------------
// Condition.Matches
// ---------------------------------------------------------------------------

func TestCondition_Matches_Nil(t *testing.T) {
	var c *Condition
	if c.Matches(fmt("banner"), false, false) {
		t.Fatal("nil condition should not match")
	}
}

func TestCondition_Matches_EmptyConditionMatchesAll(t *testing.T) {
	c := &Condition{}
	for _, tc := range []struct{ intr, push bool }{
		{false, false}, {true, false}, {false, true}, {true, true},
	} {
		if !c.Matches(fmt("native"), tc.intr, tc.push) {
			t.Errorf("empty condition must match intr=%v push=%v", tc.intr, tc.push)
		}
	}
}

func TestCondition_Matches_FormatFilter(t *testing.T) {
	c := &Condition{Formats: []string{"native", "banner"}}
	if !c.Matches(fmt("native"), false, false) {
		t.Error("native should match")
	}
	if !c.Matches(fmt("banner"), false, false) {
		t.Error("banner should match")
	}
	if c.Matches(fmt("video"), false, false) {
		t.Error("video should not match")
	}

	cStar := &Condition{Formats: []string{"*"}}
	if !cStar.Matches(fmt("direct"), false, false) {
		t.Error(`condition formats ["*"] should match direct`)
	}
	if !cStar.Matches(fmt("native"), false, false) {
		t.Error(`condition formats ["*"] should match native`)
	}
}

func TestCondition_Matches_Interstitial(t *testing.T) {
	tests := []struct {
		flag   Aplicable
		isIntr bool
		want   bool
	}{
		{Any, false, true},
		{Any, true, true},
		{Include, true, true},
		{Include, false, false},
		{Exclude, false, true},
		{Exclude, true, false},
	}
	for _, tc := range tests {
		c := &Condition{Interstitial: tc.flag}
		got := c.Matches(fmt("banner"), tc.isIntr, false)
		if got != tc.want {
			t.Errorf("Interstitial=%v isIntr=%v: got %v want %v", tc.flag, tc.isIntr, got, tc.want)
		}
	}
}

func TestCondition_Matches_Push(t *testing.T) {
	tests := []struct {
		flag   Aplicable
		isPush bool
		want   bool
	}{
		{Any, false, true},
		{Any, true, true},
		{Include, true, true},
		{Include, false, false},
		{Exclude, false, true},
		{Exclude, true, false},
	}
	for _, tc := range tests {
		c := &Condition{Push: tc.flag}
		got := c.Matches(fmt("banner"), false, tc.isPush)
		if got != tc.want {
			t.Errorf("Push=%v isPush=%v: got %v want %v", tc.flag, tc.isPush, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// RTBRules format support
// ---------------------------------------------------------------------------

func TestRTBRules_IsFormatSupport(t *testing.T) {
	r := &RTBRules{Formats: []string{"banner", "native"}}
	if !r.IsFormatSupport(fmt("banner")) {
		t.Error("banner should be supported")
	}
	if r.IsFormatSupport(fmt("video")) {
		t.Error("video should not be supported")
	}

	// Empty Formats → no restriction, all formats accepted.
	rAll := &RTBRules{}
	if !rAll.IsFormatSupport(fmt("anything")) {
		t.Error("empty formats list: all formats should be accepted")
	}

	// "*" → all formats accepted.
	rStar := &RTBRules{Formats: []string{"*"}}
	if !rStar.IsFormatSupport(fmt("direct")) {
		t.Error(`formats ["*"]: direct should be accepted`)
	}
	if !rStar.IsFormatSupport(fmt("banner_300x250")) {
		t.Error(`formats ["*"]: banner_300x250 should be accepted`)
	}
}

func TestRTBRules_IsInterstitialSupport(t *testing.T) {
	r := &RTBRules{InterstitialFormats: []string{"proxy"}}
	if !r.IsInterstitialSupport(fmt("proxy")) {
		t.Error("proxy interstitial should be supported")
	}
	if r.IsInterstitialSupport(fmt("banner")) {
		t.Error("banner interstitial should not be supported")
	}

	// Empty InterstitialFormats → not supported (no explicit list).
	rEmpty := &RTBRules{}
	if rEmpty.IsInterstitialSupport(fmt("proxy")) {
		t.Error("empty interstitial_formats: should not be supported")
	}
}

func TestRTBRules_IsPushSupport(t *testing.T) {
	r := &RTBRules{PushFormats: []string{"native"}}
	if !r.IsPushSupport(fmt("native")) {
		t.Error("native push should be supported")
	}
	if r.IsPushSupport(fmt("banner")) {
		t.Error("banner push should not be supported")
	}
}

// ---------------------------------------------------------------------------
// RTBRules.AdjustImpression
// ---------------------------------------------------------------------------

func TestRTBRules_AdjustImpression_MatchingRule(t *testing.T) {
	ext := map[string]any{"type": "webpush"}
	r := &RTBRules{
		Rules: []*RuleItem{
			{
				Condition: Condition{
					Formats: []string{"native"},
					Push:    Include,
				},
				Config: RuleConfig{Ext: ext},
			},
		},
	}

	target := &stubTarget{}
	if err := r.AdjustImpression(target, imp(false, true), fmt("native")); err != nil {
		t.Fatal(err)
	}
	if target.ext == nil {
		t.Fatal("ext should be set for matching rule")
	}
	if target.ext["type"] != "webpush" {
		t.Errorf("unexpected ext: %v", target.ext)
	}
}

func TestRTBRules_AdjustImpression_NoMatch(t *testing.T) {
	r := &RTBRules{
		Rules: []*RuleItem{
			{
				Condition: Condition{Formats: []string{"native"}, Push: Include},
				Config:    RuleConfig{Ext: map[string]any{"type": "webpush"}},
			},
		},
	}

	target := &stubTarget{}
	// push=false → condition Push:Include won't match
	if err := r.AdjustImpression(target, imp(false, false), fmt("native")); err != nil {
		t.Fatal(err)
	}
	if target.ext != nil {
		t.Errorf("ext should not be set for non-matching rule, got: %v", target.ext)
	}
}

// ---------------------------------------------------------------------------
// resolveFieldPath
// ---------------------------------------------------------------------------

func TestResolveFieldPath_SimpleKey(t *testing.T) {
	data := map[string]any{"title": "hello"}
	if got := resolveFieldPath(data, "title"); got != "hello" {
		t.Errorf("got %v", got)
	}
}

func TestResolveFieldPath_NestedPath(t *testing.T) {
	data := map[string]any{
		"images": map[string]any{
			"main": map[string]any{"url": "https://example.com/img.png"},
		},
	}
	got := resolveFieldPath(data, "images/main/url")
	if got != "https://example.com/img.png" {
		t.Errorf("got %v", got)
	}
}

func TestResolveFieldPath_MissingKey(t *testing.T) {
	data := map[string]any{"title": "hello"}
	if got := resolveFieldPath(data, "missing"); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestResolveFieldPath_IntermediateNotMap(t *testing.T) {
	data := map[string]any{"title": "hello"}
	if got := resolveFieldPath(data, "title/sub"); got != nil {
		t.Errorf("expected nil when intermediate is not a map, got %v", got)
	}
}

func TestResolveFieldPath_EmptyField(t *testing.T) {
	data := map[string]any{"k": "v"}
	if got := resolveFieldPath(data, ""); got != nil {
		t.Errorf("expected nil for empty field, got %v", got)
	}
}

func TestResolveFieldPath_NilData(t *testing.T) {
	if got := resolveFieldPath(nil, "key"); got != nil {
		t.Errorf("expected nil for nil data, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// MapResponse.Mapping
// ---------------------------------------------------------------------------

func TestMapResponse_Mapping_CollectsValues(t *testing.T) {
	mr := &MapResponse{
		Assets: []MapResponseAsset{
			{ID: 1, Name: "title", Field: "title"},
			{ID: 2, Name: "img", Field: "images/main"},
			{ID: 3, Name: "missing", Field: "no_such_key"},
		},
	}
	data := map[string]any{
		"title":  "Ad Title",
		"images": map[string]any{"main": "https://img.example.com/"},
	}

	collected := map[string]any{}
	err := mr.Mapping(data, func(name string, value any) error {
		collected[name] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if collected["title"] != "Ad Title" {
		t.Errorf("title: got %v", collected["title"])
	}
	if collected["img"] != "https://img.example.com/" {
		t.Errorf("img: got %v", collected["img"])
	}
	if collected["missing"] != nil {
		t.Errorf("missing: expected nil, got %v", collected["missing"])
	}
}

func TestMapResponse_Mapping_StopsOnError(t *testing.T) {
	mr := &MapResponse{
		Assets: []MapResponseAsset{
			{ID: 1, Name: "a", Field: "a"},
			{ID: 2, Name: "b", Field: "b"},
		},
	}
	sentinel := errors.New("stop")
	calls := 0
	err := mr.Mapping(map[string]any{"a": 1, "b": 2}, func(_ string, _ any) error {
		calls++
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call before stop, got %d", calls)
	}
}

func TestMapResponse_Mapping_NilReceiver(t *testing.T) {
	var mr *MapResponse
	if err := mr.Mapping(nil, func(_ string, _ any) error {
		t.Fatal("should not be called")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestMapResponse_Write_FlatAndNested(t *testing.T) {
	mr := &MapResponse{
		Assets: []MapResponseAsset{
			{Name: "title", Field: "title"},
			{Name: "main", Field: "image"},
			{Name: "nested", Field: "images/main/url"},
			{Name: "skip", Field: "missing"},
		},
	}
	dst := map[string]any{}
	values := map[string]any{
		"title":  "Hello",
		"main":   "https://img.example/a.png",
		"nested": "https://img.example/b.png",
		"skip":   nil,
	}
	if err := mr.Write(dst, func(name string) any { return values[name] }); err != nil {
		t.Fatal(err)
	}
	if dst["title"] != "Hello" {
		t.Errorf("title: %v", dst["title"])
	}
	if dst["image"] != "https://img.example/a.png" {
		t.Errorf("image: %v", dst["image"])
	}
	images, ok := dst["images"].(map[string]any)
	if !ok {
		t.Fatalf("images not a map: %v", dst["images"])
	}
	main, ok := images["main"].(map[string]any)
	if !ok {
		t.Fatalf("images.main not a map: %v", images["main"])
	}
	if main["url"] != "https://img.example/b.png" {
		t.Errorf("nested url: %v", main["url"])
	}
	if _, ok := dst["missing"]; ok {
		t.Error("nil value should not be written")
	}
}

func TestMapResponse_Write_NilReceiver(t *testing.T) {
	var mr *MapResponse
	if err := mr.Write(map[string]any{}, func(string) any { return "x" }); err != nil {
		t.Fatal(err)
	}
}
