package banner

import (
	"strings"

	"github.com/geniusrabbit/adcorelib/admodels/types"
)

// bannerFormatType infers the FormatType for a banner ad from its markup string.
// URLs and iframe markup are treated as proxy (FormatProxyType); everything else
// is treated as an inline HTML banner (FormatBannerType).
func bannerFormatType(markup string) types.FormatType {
	if strings.HasPrefix(markup, "http://") ||
		strings.HasPrefix(markup, "https://") ||
		(strings.HasPrefix(markup, "//") && !strings.ContainsAny(markup, "\n\t")) ||
		strings.Contains(markup, "<iframe") {
		return types.FormatProxyType
	}
	return types.FormatBannerType
}
