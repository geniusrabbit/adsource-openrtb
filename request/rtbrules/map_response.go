package rtbrules

import (
	"strings"
	"sync"
)

// MapResponseAsset maps an OpenRTB native asset ID to a named content field.
// Field supports nested paths separated by "/" (e.g. "images/main/url").
// The parsed path segments are cached on first use to avoid repeated allocations.
type MapResponseAsset struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Field    string   `json:"field"`
	segments []string // pre-split Field; populated once by MapResponse.prepare
}

// MapResponse defines how native response assets are mapped to content fields.
type MapResponse struct {
	Assets []MapResponseAsset `json:"assets,omitempty"`
	once   sync.Once
}

// prepare splits every asset's Field path exactly once (lazy, thread-safe).
func (mr *MapResponse) prepare() {
	mr.once.Do(func() {
		for i := range mr.Assets {
			if mr.Assets[i].Field != "" {
				mr.Assets[i].segments = strings.Split(mr.Assets[i].Field, "/")
			}
		}
	})
}

// Mapping iterates over all assets, resolves each asset's Field path inside
// data using pre-split segments, and calls fnk with the asset and the resolved
// value (nil when the path does not exist or leads through a non-map node).
// Segment splitting is done once on the first call; subsequent calls are
// allocation-free for the path traversal itself.
// Iteration stops on the first non-nil error returned by fnk.
func (mr *MapResponse) Mapping(data map[string]any, fnk func(string, any) error) error {
	if mr == nil {
		return nil
	}
	mr.prepare()
	for i := range mr.Assets {
		asset := &mr.Assets[i]
		if err := fnk(asset.Name, resolveSegments(data, asset.segments)); err != nil {
			return err
		}
	}
	return nil
}

// HasAssets returns true if the MapResponse has any assets defined.
func (mr *MapResponse) HasAssets() bool {
	return mr != nil && len(mr.Assets) > 0
}

// resolveSegments walks data following pre-split path segments and returns
// the terminal value, or nil if any segment is missing or an intermediate
// node is not a map[string]any.
func resolveSegments(data map[string]any, segments []string) any {
	if len(data) == 0 || len(segments) == 0 {
		return nil
	}
	current := any(data)
	for _, seg := range segments {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = m[seg]
		if !ok {
			return nil
		}
	}
	return current
}

// resolveFieldPath is a convenience wrapper that splits field on "/" and
// delegates to resolveSegments. Prefer pre-split segments (via Mapping) in
// hot paths to avoid the Split allocation.
func resolveFieldPath(data map[string]any, field string) any {
	if len(data) == 0 || field == "" {
		return nil
	}
	return resolveSegments(data, strings.Split(field, "/"))
}
