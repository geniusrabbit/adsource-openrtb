package rtbrules

import (
	"slices"
	"time"

	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/platform/info"
)

// Meta holds descriptive metadata for an RTBRules set.
// Fields are informational and may be exposed via API.
type Meta struct {
	Title       string               `json:"title,omitempty"`
	Description string               `json:"description,omitempty"`
	Version     string               `json:"version,omitempty"`
	ModifiedAt  time.Time            `json:"modified_at,omitempty"`
	Docs        []info.Documentation `json:"docs,omitempty"`
}

type Condition struct {
	Formats      []string  `json:"formats,omitempty"`
	Interstitial Aplicable `json:"interstitial,omitempty"`
	Push         Aplicable `json:"push,omitempty"`
}

func (c *Condition) Matches(format *types.Format, isIntr, isPush bool) bool {
	if c == nil {
		return false
	}
	if len(c.Formats) > 0 && !containsFormat(c.Formats, format.Codename) {
		return false
	}
	if c.Interstitial != Any {
		if (c.Interstitial == Include && !isIntr) || (c.Interstitial == Exclude && isIntr) {
			return false
		}
	}
	if c.Push != Any {
		if (c.Push == Include && !isPush) || (c.Push == Exclude && isPush) {
			return false
		}
	}
	return true
}

// RuleConfig holds extra parameters passed to the ad exchange for a matched rule.
type RuleConfig struct {
	Ext             map[string]any `json:"ext,omitempty"`
	NoRequestObject bool           `json:"no_request_object,omitempty"`
}

// RuleItem represents a single rule in the RTBRules set, consisting of a condition, configuration, and optional response mapping.
type RuleItem struct {
	Condition   Condition    `json:"condition"`
	Config      RuleConfig   `json:"config"`
	MapResponse *MapResponse `json:"map_response,omitempty"`
}

// RTBRules defines a set of rules for handling OpenRTB requests.
// Interstitial and push are considered supported when their respective
// format lists are explicitly non-empty.
type RTBRules struct {
	Meta                Meta        `json:"meta,omitempty"`
	Formats             []string    `json:"formats,omitempty"`
	InterstitialFormats []string    `json:"interstitial_formats,omitempty"`
	PushFormats         []string    `json:"push_formats,omitempty"`
	Rules               []*RuleItem `json:"rules,omitempty"`
}

// containsFormat reports whether the format list accepts the given codename.
// An empty list or a "*" entry matches any format.
func containsFormat(list []string, codename string) bool {
	return len(list) == 0 || slices.Contains(list, "*") || slices.Contains(list, codename)
}

// IsFormatSupport returns true when the format codename is listed in Formats.
// An empty Formats list or a "*" entry means no restriction — all formats are accepted.
func (r *RTBRules) IsFormatSupport(format *types.Format) bool {
	return r != nil && containsFormat(r.Formats, format.Codename)
}

// IsInterstitialSupport returns true when InterstitialFormats is non-empty and
// contains the given format codename.
func (r *RTBRules) IsInterstitialSupport(format *types.Format) bool {
	return r != nil && len(r.InterstitialFormats) > 0 && slices.Contains(r.InterstitialFormats, format.Codename)
}

// IsPushSupport returns true when PushFormats is non-empty and contains the
// given format codename.
func (r *RTBRules) IsPushSupport(format *types.Format) bool {
	return r != nil && len(r.PushFormats) > 0 && slices.Contains(r.PushFormats, format.Codename)
}

// HasMappingRules checks if there are any mapping rules defined in the RTBRules configuration.
func (r *RTBRules) HasMappingRules() bool {
	if r == nil {
		return false
	}
	for _, rule := range r.Rules {
		if rule.MapResponse != nil && rule.MapResponse.HasAssets() {
			return true
		}
	}
	return false
}

// ApplyRules iterates over all rules, applying the provided function to each rule that matches the given format and impression type.
func (r *RTBRules) ApplyRules(format *types.Format, isIntr, isPush bool, fn func(rule *RuleItem) error) error {
	if r == nil {
		return nil
	}
	for _, rule := range r.Rules {
		if rule.Condition.Matches(format, isIntr, isPush) {
			if err := fn(rule); err != nil {
				return err
			}
		}
	}
	return nil
}

// NoRequestObject returns true if the first matching rule instructs the builder
// to omit the native request object from the bid request.
func (r *RTBRules) NoRequestObject(format *types.Format, isIntr, isPush bool) bool {
	if r == nil {
		return false
	}
	for _, rule := range r.Rules {
		if rule.Condition.Matches(format, isIntr, isPush) {
			return rule.Config.NoRequestObject
		}
	}
	return false
}

// AdjustImpression applies the first matching rule's configuration to the target impression, if any rule matches the given format and impression type.
func (r *RTBRules) AdjustImpression(target TargetImpression, imp *adtype.Impression, format *types.Format) error {
	if r == nil {
		return nil
	}
	for _, rule := range r.Rules {
		if rule.Condition.Matches(format, imp.IsInterstitial(), imp.IsPush()) {
			target.SetExt(rule.Config.Ext)
		}
	}
	return nil
}

var _ RTBRuler = (*RTBRules)(nil)
