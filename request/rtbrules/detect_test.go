package rtbrules

import (
	"encoding/json"
	"testing"
)

func TestRTBRules_Detect_NilReceiver(t *testing.T) {
	var r *RTBRules
	if r.Detect(RequestSignal{Ext: map[string]any{"type": "pop"}}) != nil {
		t.Fatal("nil rules should not detect")
	}
}

func TestRTBRules_Detect_EmptyExtSkipped(t *testing.T) {
	r := &RTBRules{
		Rules: []*RuleItem{
			{
				Condition: Condition{Formats: []string{"native"}},
				Config:    RuleConfig{},
			},
		},
	}
	if got := r.Detect(RequestSignal{Ext: map[string]any{"type": "webpush"}}); got != nil {
		t.Fatalf("empty Config.Ext must not match, got %+v", got)
	}
}

func TestRTBRules_Detect_ExtSubsetNumeric(t *testing.T) {
	r := &RTBRules{
		Rules: []*RuleItem{
			{
				Condition: Condition{Formats: []string{"direct"}, Interstitial: Exclude, Push: Exclude},
				Config:    RuleConfig{Ext: map[string]any{"format": 4}},
			},
		},
	}
	for _, ext := range []map[string]any{
		{"format": 4},
		{"format": float64(4)},
		{"format": json.Number("4")},
		{"format": "4"},
		{"format": 4, "extra": "ok"},
	} {
		got := r.Detect(RequestSignal{Ext: ext})
		if got == nil {
			t.Fatalf("ext %v: expected match", ext)
		}
		if len(got.FormatCodes) != 1 || got.FormatCodes[0] != "direct" {
			t.Errorf("ext %v: FormatCodes=%v", ext, got.FormatCodes)
		}
		if got.Push || got.Interstitial {
			t.Errorf("ext %v: unexpected flags push=%v instl=%v", ext, got.Push, got.Interstitial)
		}
	}
}

func TestRTBRules_Detect_InterstitialExcludeSkips(t *testing.T) {
	r := &RTBRules{
		Rules: []*RuleItem{
			{
				Condition: Condition{Formats: []string{"banner_300x250"}, Interstitial: Exclude},
				Config:    RuleConfig{Ext: map[string]any{"format": 1}},
			},
			{
				Condition: Condition{Formats: []string{"proxy"}, Interstitial: Include},
				Config:    RuleConfig{Ext: map[string]any{"format": 5}},
			},
		},
	}
	if got := r.Detect(RequestSignal{Ext: map[string]any{"format": 1}, Instl: true}); got != nil {
		t.Fatalf("instl+format=1 should skip Exclude rule, got %+v", got)
	}
	got := r.Detect(RequestSignal{Ext: map[string]any{"format": 5}})
	if got == nil {
		t.Fatal("format=5 should match")
	}
	if !got.Interstitial {
		t.Error("Interstitial Include should set Interstitial")
	}
	if got.FormatCodes[0] != "proxy" {
		t.Errorf("FormatCodes=%v", got.FormatCodes)
	}
}

func TestRTBRules_Detect_PushAndNoRequestObject(t *testing.T) {
	r := &RTBRules{
		Rules: []*RuleItem{
			{
				Condition: Condition{Formats: []string{"native"}, Push: Include, Interstitial: Exclude},
				Config:    RuleConfig{Ext: map[string]any{"type": "webpush"}, NoRequestObject: true},
			},
		},
	}
	got := r.Detect(RequestSignal{Ext: map[string]any{"type": "webpush"}})
	if got == nil {
		t.Fatal("expected webpush match")
	}
	if !got.Push || !got.NoRequestObject {
		t.Errorf("push=%v noReq=%v", got.Push, got.NoRequestObject)
	}
	if got.FormatCodes[0] != "native" {
		t.Errorf("FormatCodes=%v", got.FormatCodes)
	}
}

func TestRTBRules_Detect_NoMatch(t *testing.T) {
	r := &RTBRules{
		Rules: []*RuleItem{
			{
				Condition: Condition{Formats: []string{"direct"}},
				Config:    RuleConfig{Ext: map[string]any{"type": "pop"}},
			},
		},
	}
	if got := r.Detect(RequestSignal{Ext: map[string]any{"type": "banner"}, HasBanner: true}); got != nil {
		t.Fatalf("unexpected match: %+v", got)
	}
	if got := r.Detect(RequestSignal{}); got != nil {
		t.Fatalf("empty signal should not match: %+v", got)
	}
}

func TestExtSubset_Nested(t *testing.T) {
	need := map[string]any{"a": map[string]any{"b": 1}}
	have := map[string]any{"a": map[string]any{"b": float64(1), "c": 2}, "x": true}
	if !extSubset(need, have) {
		t.Fatal("nested subset should match")
	}
	if extSubset(need, map[string]any{"a": map[string]any{"b": 2}}) {
		t.Fatal("nested mismatch should fail")
	}
}
