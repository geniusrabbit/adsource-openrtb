//
// @project GeniusRabbit corelib 2017 - 2019, 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019, 2025
//

// Package response handles OpenRTB bid response processing.
// It provides per-format response item types (banner, direct, native, vast)
// as well as the top-level [BidResponse] that aggregates all bid items.
package response

import "errors"

// Sentinel errors for ad response validation.
var (
	// ErrInvalidAdContent is returned when the ad markup cannot be resolved to usable content.
	ErrInvalidAdContent = errors.New("invalid ad content")

	// ErrInvalidVAST is returned when the VAST document structure is not valid.
	ErrInvalidVAST = errors.New("invalid VAST response")

	// ErrUnsupportedVASTConfiguration is returned for VAST configuration errors.
	ErrUnsupportedVASTConfiguration = errors.New("unsupported VAST configuration")
)
