package native

import (
	"encoding/json"
	"testing"

	"github.com/bsm/openrtb"
	natresp "github.com/bsm/openrtb/native/response"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/billing"

	"github.com/geniusrabbit/adsource-openrtb/request/rtbrules"
	"github.com/geniusrabbit/adsource-openrtb/response/common"
)

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

// stubTarget implements adtype.Target with all-zero values (no panics).
type stubTarget struct{}

func (*stubTarget) ID() uint64                                  { return 0 }
func (*stubTarget) Codename() string                            { return "" }
func (*stubTarget) ObjectKey() string                           { return "" }
func (*stubTarget) PricingModel() types.PricingModel            { return 0 }
func (*stubTarget) AlternativeAdCode(_ string) string           { return "" }
func (*stubTarget) PurchasePrice(_ adtype.Action) billing.Money { return 0 }
func (*stubTarget) CommissionShareFactor() float64              { return 0 }
func (*stubTarget) RevenueShareReduceFactor() float64           { return 0 }
func (*stubTarget) Account() adtype.Account                     { return nil }

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// trafficStarsPushMarkup is a real-world flat TrafficStars push-notification
// bid response (non-OpenRTB-native, custom JSON format).
const trafficStarsPushMarkup = `{"id":"43ddb67e-e85e-425e-a29c-f0b16eeb2306","title":"Example title","content":"Example content","url":"https://tsyndicate.com/do2/click?","icon":"https://pxl.tsyndicate.com/api/v1/icon","image":"https://cdn.tsyndicate.com/images/example.jpeg"}`

const trafficStarsWinURL = "https://pxl.tsyndicate.com/api/v1/win?"

// trafficStarsRules mirrors rules/data/trafficstars.json for push native.
func trafficStarsRules() *rtbrules.RTBRules {
	return &rtbrules.RTBRules{
		PushFormats: []string{"native"},
		Rules: []*rtbrules.RuleItem{
			{
				Condition: rtbrules.Condition{
					Formats:      []string{"native"},
					Interstitial: rtbrules.Exclude,
					Push:         rtbrules.Include,
				},
				Config: rtbrules.RuleConfig{Ext: map[string]any{"type": "webpush"}},
				MapResponse: &rtbrules.MapResponse{
					Assets: []rtbrules.MapResponseAsset{
						{ID: 1, Name: types.FormatAssetMain, Field: "image"},
						{ID: 2, Name: types.FormatAssetLogo, Field: "icon"},
						{ID: 101, Name: "title", Field: "title"},
						{ID: 102, Name: "description", Field: "content"},
						{ID: 105, Name: "url", Field: "url"},
					},
				},
			},
		},
	}
}

// nativePushFormat is a minimal "native" format whose config assets match
// the TrafficStars push mapping (ID 1 = main image, ID 2 = logo).
func nativePushFormat() *types.Format {
	return &types.Format{
		Codename: "native",
		Config: &types.FormatConfig{
			Assets: []types.FormatFileRequirement{
				{ID: 1, Name: types.FormatAssetMain, Required: true},
				{ID: 2, Name: types.FormatAssetLogo},
			},
		},
	}
}

func pushImpression() *adtype.Impression {
	return &adtype.Impression{
		ID:     "1668566261",
		Push:   true,
		Target: &stubTarget{},
	}
}

func makeBid(adMarkup, nurl string) *openrtb.Bid {
	return &openrtb.Bid{
		ID:       "43ddb67e-e85e-425e-a29c-f0b16eeb2306",
		AdMarkup: adMarkup,
		NURL:     nurl,
		Price:    0.05,
	}
}

// openRTBNativeMarkup produces a minimal standard OpenRTB native markup with
// a title asset and click-through link.
func openRTBNativeMarkup(title, linkURL string) string {
	native := natresp.Response{
		Link: natresp.Link{URL: linkURL},
		Assets: []natresp.Asset{
			{ID: 101, Title: &natresp.Title{Text: title}},
		},
	}
	b, _ := json.Marshal(native)
	return string(b)
}

// ---------------------------------------------------------------------------
// TrafficStars push-notification mapping
// ---------------------------------------------------------------------------

var stubSrc = &adtype.SourceEmpty{}

func TestNew_TrafficStarsPush_TextFieldsMapped(t *testing.T) {
	item, err := New(nil, stubSrc,
		makeBid(trafficStarsPushMarkup, trafficStarsWinURL),
		pushImpression(), nativePushFormat(), trafficStarsRules())
	require.NoError(t, err)

	assert.Equal(t, "Example title", item.ContentItemString("title"))
	assert.Equal(t, "Example content", item.ContentItemString("description"))
	assert.Equal(t, "https://tsyndicate.com/do2/click?", item.ContentItemString("url"))
}

func TestNew_TrafficStarsPush_WinURL(t *testing.T) {
	item, err := New(nil, stubSrc,
		makeBid(trafficStarsPushMarkup, trafficStarsWinURL),
		pushImpression(), nativePushFormat(), trafficStarsRules())
	require.NoError(t, err)

	assert.Equal(t, trafficStarsWinURL, item.ContentItemString(adtype.ContentItemNotifyWinURL))
}

func TestNew_TrafficStarsPush_ImageAssetsExtracted(t *testing.T) {
	item, err := New(nil, stubSrc,
		makeBid(trafficStarsPushMarkup, trafficStarsWinURL),
		pushImpression(), nativePushFormat(), trafficStarsRules())
	require.NoError(t, err)

	assets := item.Assets()
	require.NotEmpty(t, assets, "expected file assets from mapping")

	byName := map[string]string{}
	for _, a := range assets {
		byName[a.Name] = a.URL
	}
	assert.Equal(t, "https://cdn.tsyndicate.com/images/example.jpeg", byName[types.FormatAssetMain],
		"main: should map TrafficStars 'image' field")
	assert.Equal(t, "https://pxl.tsyndicate.com/api/v1/icon", byName[types.FormatAssetLogo],
		"logo: should map TrafficStars 'icon' field")
}

func TestNew_TrafficStarsPush_MainAsset(t *testing.T) {
	item, err := New(nil, stubSrc,
		makeBid(trafficStarsPushMarkup, trafficStarsWinURL),
		pushImpression(), nativePushFormat(), trafficStarsRules())
	require.NoError(t, err)

	main := item.MainAsset()
	require.NotNil(t, main)
	assert.Equal(t, "https://cdn.tsyndicate.com/images/example.jpeg", main.URL)
	assert.Equal(t, types.FormatAssetMain, main.Name)
}

func TestNew_TrafficStarsPush_RulesNotMatchForNonPush(t *testing.T) {
	// Push=false → condition Push:Include won't fire → falls back to direct decode.
	// The flat JSON parses but has no OpenRTB title asset.
	nonPush := &adtype.Impression{ID: "imp-nopush", Push: false, Target: &stubTarget{}}

	item, err := New(nil, stubSrc, makeBid(trafficStarsPushMarkup, ""),
		nonPush, nativePushFormat(), trafficStarsRules())
	if err != nil {
		return // validation may reject the empty native response; acceptable
	}
	// Mapping was NOT applied → Data map not populated from flat JSON fields.
	assert.Empty(t, item.ContentItemString("title"),
		"title must be empty when mapping rule did not match")
	assert.Empty(t, item.ContentItemString("description"))
}

// ---------------------------------------------------------------------------
// Standard OpenRTB native path (no rules / rules without MapResponse)
// ---------------------------------------------------------------------------

func TestNew_StandardNative_NoRules(t *testing.T) {
	markup := openRTBNativeMarkup("Native Title", "https://example.com/landing")
	format := &types.Format{Codename: "native"}
	imp := &adtype.Impression{ID: "imp-1", Target: &stubTarget{}}

	item, err := New(nil, stubSrc, makeBid(markup, ""), imp, format, nil)
	require.NoError(t, err)

	assert.Equal(t, "Native Title", item.ContentItemString(types.FormatFieldTitle))
	assert.Equal(t, "https://example.com/landing", item.ContentItemString(adtype.ContentItemLink))
	assert.Equal(t, "https://example.com/landing", item.ActionURL())
}

func TestNew_StandardNative_WrappedMarkup(t *testing.T) {
	// {"native": {...}} outer wrapper — also valid per OpenRTB spec.
	inner := openRTBNativeMarkup("Wrapped Title", "https://example.com")
	wrapped := `{"native":` + inner + `}`
	format := &types.Format{Codename: "native"}
	imp := &adtype.Impression{ID: "imp-2", Target: &stubTarget{}}

	item, err := New(nil, stubSrc, makeBid(wrapped, ""), imp, format, nil)
	require.NoError(t, err)
	assert.Equal(t, "Wrapped Title", item.ContentItemString(types.FormatFieldTitle))
}

func TestNew_StandardNative_RulesWithoutMapResponse(t *testing.T) {
	// Rules present but no MapResponse → standard decode must run.
	rulesNoMap := &rtbrules.RTBRules{
		Rules: []*rtbrules.RuleItem{
			{
				Condition: rtbrules.Condition{Formats: []string{"native"}},
				Config:    rtbrules.RuleConfig{Ext: map[string]any{"k": "v"}},
			},
		},
	}
	markup := openRTBNativeMarkup("Title via rule", "https://example.com")
	format := &types.Format{Codename: "native"}
	imp := &adtype.Impression{ID: "imp-3", Target: &stubTarget{}}

	item, err := New(nil, stubSrc, makeBid(markup, ""), imp, format, rulesNoMap)
	require.NoError(t, err)
	assert.Equal(t, "Title via rule", item.ContentItemString(types.FormatFieldTitle))
}

// ---------------------------------------------------------------------------
// SetContentItem unit tests
// ---------------------------------------------------------------------------

func TestSetContentItem_Link(t *testing.T) {
	item := &ResponseBidItem{Native: &natresp.Response{}}
	require.NoError(t, item.SetContentItem(adtype.ContentItemLink, "https://link.example.com"))
	assert.Equal(t, "https://link.example.com", item.Native.Link.URL)
}

func TestSetContentItem_StoresUnknownInDataMap(t *testing.T) {
	item := &ResponseBidItem{Native: &natresp.Response{}}
	require.NoError(t, item.SetContentItem("custom_field", "custom_value"))
	require.NotNil(t, item.Data)
	assert.Equal(t, "custom_value", item.Data["custom_field"])
}

func TestSetContentItem_WinURL(t *testing.T) {
	bid := makeBid("{}", "")
	item := &ResponseBidItem{
		Native: &natresp.Response{},
		BaseBidItem: common.BaseBidItem{
			Bid: bid,
			Imp: &adtype.Impression{Target: &stubTarget{}},
		},
	}
	require.NoError(t, item.SetContentItem(adtype.ContentItemNotifyWinURL, "https://win.example.com"))
	assert.Equal(t, "https://win.example.com", bid.NURL)
}

func TestSetContentItem_DisplayURL(t *testing.T) {
	bid := makeBid("{}", "")
	item := &ResponseBidItem{
		Native: &natresp.Response{},
		BaseBidItem: common.BaseBidItem{
			Bid: bid,
			Imp: &adtype.Impression{Target: &stubTarget{}},
		},
	}
	require.NoError(t, item.SetContentItem(adtype.ContentItemNotifyDisplayURL, "https://burl.example.com"))
	assert.Equal(t, "https://burl.example.com", bid.BURL)
}

func TestSetContentItem_KnownAssetCreatesFileAsset(t *testing.T) {
	// Verify that SetContentItem for a format-configured asset name
	// produces a file asset rather than storing in Data map.
	item, err := New(nil, stubSrc,
		makeBid(trafficStarsPushMarkup, ""), pushImpression(), nativePushFormat(), trafficStarsRules())
	require.NoError(t, err)

	before := len(item.Assets())
	require.NoError(t, item.SetContentItem(types.FormatAssetMain, "https://new.example.com/img.jpg"))
	assert.Equal(t, before+1, len(item.Assets()), "a file asset should be appended")
	assert.Equal(t, "https://new.example.com/img.jpg", item.Assets()[len(item.Assets())-1].URL)
	// Not stored in Data map
	assert.Nil(t, item.Data[types.FormatAssetMain])
}

// ---------------------------------------------------------------------------
// ContentItem fallback chain
// ---------------------------------------------------------------------------

func TestContentItem_FallbackToNativeAssets_Title(t *testing.T) {
	markup := openRTBNativeMarkup("Fallback title", "https://fb.example.com")
	format := &types.Format{Codename: "native"}
	imp := &adtype.Impression{ID: "imp-fb", Target: &stubTarget{}}

	item, err := New(nil, stubSrc, makeBid(markup, ""), imp, format, nil)
	require.NoError(t, err)

	// No V2/V3 native request attached → Data is nil → falls back to Native.Assets scan.
	assert.Equal(t, "Fallback title", item.ContentItemString(types.FormatFieldTitle))
}

func TestContentItem_DataMapTakesPrecedence(t *testing.T) {
	markup := openRTBNativeMarkup("Native title", "https://example.com")
	format := &types.Format{Codename: "native"}
	imp := &adtype.Impression{ID: "imp-prio", Target: &stubTarget{}}

	item, err := New(nil, stubSrc, makeBid(markup, ""), imp, format, nil)
	require.NoError(t, err)

	// Manually inject a value into Data that shadows the native asset.
	item.Data = map[string]any{types.FormatFieldTitle: "Override title"}
	assert.Equal(t, "Override title", item.ContentItemString(types.FormatFieldTitle),
		"Data map value must take precedence over Native.Assets scan")
}
