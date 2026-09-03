package rtbrules

import (
	"encoding/json"
	"strconv"
)

// RequestSignal is the inbound OpenRTB impression shape used to invert
// format→Ext rules (DSP ingress).
type RequestSignal struct {
	Ext                            map[string]any
	HasBanner, HasNative, HasVideo bool
	Instl                          bool
}

// Detection is the inverted match: partner Ext → format codes and flags.
type Detection struct {
	FormatCodes     []string
	Interstitial    bool
	Push            bool
	NoRequestObject bool
	Rule            *RuleItem
}

// Detect returns the first rule whose Config.Ext is a subset of the incoming
// impression Ext. Rules with empty Ext are skipped (empty is a subset of
// everything and would otherwise match every request).
//
// Interstitial Exclude + instl skips the rule. Push and Interstitial Include
// are outputs. No match returns nil.
func (r *RTBRules) Detect(sig RequestSignal) *Detection {
	if r == nil {
		return nil
	}
	for _, rule := range r.Rules {
		if rule == nil || len(rule.Config.Ext) == 0 {
			continue
		}
		if !extSubset(rule.Config.Ext, sig.Ext) {
			continue
		}
		if rule.Condition.Interstitial == Exclude && sig.Instl {
			continue
		}
		codes := rule.Condition.Formats
		if len(codes) > 0 {
			codes = append([]string(nil), codes...)
		}
		return &Detection{
			FormatCodes:     codes,
			Interstitial:    sig.Instl || rule.Condition.Interstitial == Include,
			Push:            rule.Condition.Push == Include,
			NoRequestObject: rule.Config.NoRequestObject,
			Rule:            rule,
		}
	}
	return nil
}

// extSubset reports whether every key in need is present in have with an
// equal value (JSON-number-tolerant). An empty need is not a subset.
func extSubset(need, have map[string]any) bool {
	if len(need) == 0 {
		return false
	}
	for k, nv := range need {
		hv, ok := have[k]
		if !ok || !extValuesEqual(nv, hv) {
			return false
		}
	}
	return true
}

func extValuesEqual(a, b any) bool {
	if ma, ok := asMap(a); ok {
		mb, ok := asMap(b)
		if !ok {
			return false
		}
		if len(ma) == 0 {
			return true
		}
		return extSubset(ma, mb)
	}
	if fa, ok := asFloat(a); ok {
		if fb, ok := asFloat(b); ok {
			return fa == fb
		}
	}
	return stringify(a) == stringify(b)
}

func asMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok && m != nil
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func stringify(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
