package requestoptions

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/geniusrabbit/adcorelib/admodels/types"
)

func TestWithBidFloor_Negative(t *testing.T) {
	var opts BidRequestRTBOptions
	WithBidFloor(-5.0)(&opts)
	assert.Equal(t, 0.0, opts.BidFloor, "negative bid floor should be clamped to 0")
}

func TestWithFormatFilter(t *testing.T) {
	called := false
	filterFn := func(f *types.Format) bool {
		called = true
		return true
	}
	var opts BidRequestRTBOptions
	WithFormatFilter(filterFn)(&opts)
	opts.FormatFilter(&types.Format{})
	assert.True(t, called, "FormatFilter should have been called")
}

func TestWithRTBOpenNativeVersion(t *testing.T) {
	var opts BidRequestRTBOptions
	WithRTBOpenNativeVersion("1.2")(&opts)
	assert.Equal(t, "1.2", opts.OpenNativeVer())
}

func TestBidRequestRTBOptions_DefaultCurrency(t *testing.T) {
	var opts BidRequestRTBOptions
	assert.Equal(t, []string{"USD"}, opts.Currencies())
}
