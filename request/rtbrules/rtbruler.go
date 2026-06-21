package rtbrules

import (
	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
)

type TargetImpression interface {
	SetExt(ext map[string]any)
}

type RTBRuler interface {
	IsFormatSupport(format *types.Format) bool
	IsInterstitialSupport(format *types.Format) bool
	IsPushSupport(format *types.Format) bool
	HasMappingRules() bool
	NoRequestObject(format *types.Format, isIntr, isPush bool) bool
	ApplyRules(format *types.Format, isIntr, isPush bool, fn func(rule *RuleItem) error) error
	AdjustImpression(target TargetImpression, imp *adtype.Impression, format *types.Format) error
}
