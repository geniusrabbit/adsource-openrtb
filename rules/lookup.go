package rules

import "github.com/geniusrabbit/adsource-openrtb/request/rtbrules"

// ByName returns the compiled rule set for name, or the default set when the
// name is empty or unknown.
func ByName(name string) *rtbrules.RTBRules {
	if r := Rules[name]; r != nil {
		return r
	}
	return Rules["default"]
}
