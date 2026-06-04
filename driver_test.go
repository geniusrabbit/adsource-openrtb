package adsourceopenrtb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bsm/openrtb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adquery/bidrequest"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/net/httpclient"

	rtbreq "github.com/geniusrabbit/adsource-openrtb/response/requester"
)

// ─── Mock HTTP infrastructure ────────────────────────────────────────────────

type mockRequest struct {
	headers map[string]string
}

func (r *mockRequest) SetHeader(key, value string) {
	if r.headers == nil {
		r.headers = make(map[string]string)
	}
	r.headers[key] = value
}

type mockResponse struct {
	statusCode int
	body       io.Reader
}

func (r *mockResponse) StatusCode() int { return r.statusCode }
func (r *mockResponse) Body() io.Reader { return r.body }
func (r *mockResponse) Close() error    { return nil }

type mockHTTPClient struct {
	// Callback invoked by Do(); set per test case.
	doFn func(req httpclient.Request) (httpclient.Response, error)
}

func (c *mockHTTPClient) Request(_, _ string, _ io.Reader) (httpclient.Request, error) {
	return &mockRequest{}, nil
}

func (c *mockHTTPClient) Do(req httpclient.Request) (httpclient.Response, error) {
	if c.doFn != nil {
		return c.doFn(req)
	}
	return &mockResponse{statusCode: http.StatusNoContent}, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func makeSource(protocol string) *admodels.RTBSource {
	return &admodels.RTBSource{
		ID:          42,
		Protocol:    protocol,
		URL:         "http://example.com/rtb",
		Method:      "POST",
		RequestType: RequestTypeJSON,
	}
}

func makeDriver(t *testing.T, src *admodels.RTBSource, cli httpclient.Driver) *driver {
	t.Helper()
	requester, err := rtbreq.NewHttpRTBRequester(src, cli)
	require.NoError(t, err)
	d, err := newDriver(context.Background(), src, requester)
	require.NoError(t, err)
	requester.SetSource(d)
	return d
}

func makeBidRequest() *bidrequest.BidRequest {
	imps := []*adtype.Impression{makeBannerImpression()}
	return &bidrequest.BidRequest{
		IDVal: "test-bid-id",
		Ctx:   context.Background(),
		Imps:  imps,
	}
}

// makeBannerImpression creates a minimal banner impression with one banner format.
func makeBannerImpression() *adtype.Impression {
	formats := types.MockFormats()
	imp := &adtype.Impression{
		ID:     "imp-1",
		Width:  300,
		Height: 250,
		Target: &adtype.TargetEmpty{},
	}
	var bannerFormats []*types.Format
	for _, f := range formats {
		if f.IsBanner() {
			bannerFormats = append(bannerFormats, f)
			break
		}
	}
	if len(bannerFormats) > 0 {
		imp.InitFormats(types.NewSimpleFormatAccessor(bannerFormats))
	}
	return imp
}

// rtbNoBidResponse returns a marshalled OpenRTB BidResponse with no SeatBid.
func rtbNoBidJSON() io.Reader {
	data, _ := json.Marshal(openrtb.BidResponse{ID: "resp-1"})
	return bytes.NewReader(data)
}

// rtbBidJSON returns a marshalled OpenRTB BidResponse with one banner bid.
// The ImpID matches the impression ID returned by makeBannerImpression().
func rtbBidJSON() io.Reader {
	resp := openrtb.BidResponse{
		ID: "resp-1",
		SeatBid: []openrtb.SeatBid{
			{
				Bid: []openrtb.Bid{
					{ID: "bid-1", ImpID: "imp-1", Price: 1.5, AdMarkup: "<img/>"},
				},
			},
		},
	}
	data, _ := json.Marshal(resp)
	return bytes.NewReader(data)
}

// ─── newDriver validation ────────────────────────────────────────────────────

func TestNewDriver_NilSource(t *testing.T) {
	_, err := newDriver(context.Background(), nil, &rtbreq.MockRTBRequester{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNilSource)
}

func TestNewDriver_NilHTTPClient(t *testing.T) {
	_, err := newDriver(context.Background(), makeSource("openrtb"), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNilHTTPClient)
}

func TestNewDriver_OK(t *testing.T) {
	d := makeDriver(t, makeSource("openrtb"), &mockHTTPClient{})
	assert.Equal(t, uint64(42), d.ID())
	assert.Equal(t, "openrtb", d.Protocol())
}

func TestNewDriver_MinimalWeightDefault(t *testing.T) {
	src := makeSource("openrtb")
	src.MinimalWeight = 0 // should be bumped to defaultMinWeight
	d := makeDriver(t, src, &mockHTTPClient{})
	assert.Equal(t, defaultMinWeight, d.Weight())
}

// ─── driver.Test ─────────────────────────────────────────────────────────────

func TestDriver_Test_NoRPSLimit(t *testing.T) {
	d := makeDriver(t, makeSource("openrtb"), &mockHTTPClient{})
	// source.Test always returns true when there are no filters; no RPS configured
	req := makeBidRequest()
	assert.True(t, d.Test(req))
}

func TestDriver_Test_RPSExceeded(t *testing.T) {
	src := makeSource("openrtb")
	src.RPS = 1
	d := makeDriver(t, src, &mockHTTPClient{})
	req := makeBidRequest()

	// First call should pass (counter == 0 < RPS 1); second should be rejected
	d.Test(req) // warmup: sets lastRequestTime
	d.rpsCurrent.Set(1)
	assert.False(t, d.Test(req), "RPS limit should block the second request")
}

// ─── driver.Bid – HTTP status handling ───────────────────────────────────────

func TestDriver_Bid_HTTPError(t *testing.T) {
	cli := &mockHTTPClient{
		doFn: func(_ httpclient.Request) (httpclient.Response, error) {
			return nil, errors.New("connection refused")
		},
	}
	d := makeDriver(t, makeSource("openrtb"), cli)
	resp := d.Bid(makeBidRequest())
	require.NotNil(t, resp)
	assert.Error(t, resp.Error())
}

func TestDriver_Bid_204NoBid(t *testing.T) {
	cli := &mockHTTPClient{
		doFn: func(_ httpclient.Request) (httpclient.Response, error) {
			return &mockResponse{statusCode: http.StatusNoContent}, nil
		},
	}
	d := makeDriver(t, makeSource("openrtb"), cli)
	resp := d.Bid(makeBidRequest())
	require.NotNil(t, resp)
	assert.ErrorIs(t, resp.Error(), ErrResponseNoBid)
}

func TestDriver_Bid_404NoBid(t *testing.T) {
	cli := &mockHTTPClient{
		doFn: func(_ httpclient.Request) (httpclient.Response, error) {
			return &mockResponse{statusCode: http.StatusNotFound}, nil
		},
	}
	d := makeDriver(t, makeSource("openrtb"), cli)
	resp := d.Bid(makeBidRequest())
	require.NotNil(t, resp)
	assert.ErrorIs(t, resp.Error(), ErrResponseNoBid)
}

func TestDriver_Bid_500InvalidStatus(t *testing.T) {
	cli := &mockHTTPClient{
		doFn: func(_ httpclient.Request) (httpclient.Response, error) {
			return &mockResponse{statusCode: http.StatusInternalServerError}, nil
		},
	}
	d := makeDriver(t, makeSource("openrtb"), cli)
	resp := d.Bid(makeBidRequest())
	require.NotNil(t, resp)
	assert.ErrorIs(t, resp.Error(), ErrInvalidResponseStatus)

	var httpErr *HTTPStatusError
	require.True(t, errors.As(resp.Error(), &httpErr), "should be HTTPStatusError")
	assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
}

func TestDriver_Bid_200EmptyBody_NoBid(t *testing.T) {
	cli := &mockHTTPClient{
		doFn: func(_ httpclient.Request) (httpclient.Response, error) {
			return &mockResponse{statusCode: http.StatusOK, body: rtbNoBidJSON()}, nil
		},
	}
	d := makeDriver(t, makeSource("openrtb"), cli)
	resp := d.Bid(makeBidRequest())
	require.NotNil(t, resp)
	// Empty SeatBid → no ads, error == nil or ErrResponseNoBid (empty response)
	assert.Empty(t, resp.Ads())
}

func TestDriver_Bid_200WithBids(t *testing.T) {
	cli := &mockHTTPClient{
		doFn: func(_ httpclient.Request) (httpclient.Response, error) {
			return &mockResponse{statusCode: http.StatusOK, body: rtbBidJSON()}, nil
		},
	}
	d := makeDriver(t, makeSource("openrtb"), cli)
	resp := d.Bid(makeBidRequest())
	require.NotNil(t, resp)
	// Response parsed successfully; may contain ads or not depending on impression mapping.
	// At minimum, no transport/protocol error should be returned.
	assert.NoError(t, resp.Error())
}

// ─── Typed error – HTTPStatusError ───────────────────────────────────────────

func TestHTTPStatusError_Is(t *testing.T) {
	err := &HTTPStatusError{Code: http.StatusBadGateway}
	assert.ErrorIs(t, err, ErrInvalidResponseStatus)
	assert.Equal(t, "invalid response status: 502 Bad Gateway", err.Error())
}

// ─── Typed error – UnsupportedTypeError / UndefinedTypeError ─────────────────

func TestUnsupportedTypeError_Is(t *testing.T) {
	err := &UnsupportedTypeError{TypeName: "xml"}
	assert.ErrorIs(t, err, ErrUnsupportedRequestType)
	assert.True(t, strings.Contains(err.Error(), "xml"))
}

func TestUndefinedTypeError_Is(t *testing.T) {
	err := &UndefinedTypeError{TypeName: "unknown"}
	assert.ErrorIs(t, err, ErrUndefinedRequestType)
	assert.True(t, strings.Contains(err.Error(), "unknown"))
}

// ─── driver.unmarshal – unsupported request type ──────────────────────────────

// ─── driver metadata ─────────────────────────────────────────────────────────

func TestDriver_PriceCorrectionReduceFactor(t *testing.T) {
	src := makeSource("openrtb")
	src.PriceCorrectionReduce = 0.1
	d := makeDriver(t, src, &mockHTTPClient{})
	assert.InDelta(t, 0.1, d.PriceCorrectionReduceFactor(), 1e-9)
}

func TestDriver_RequestStrategy(t *testing.T) {
	d := makeDriver(t, makeSource("openrtb"), &mockHTTPClient{})
	assert.Equal(t, adtype.AsynchronousRequestStrategy, d.RequestStrategy())
}

func TestDriver_AccountID_NilAccount(t *testing.T) {
	d := makeDriver(t, makeSource("openrtb"), &mockHTTPClient{})
	assert.Equal(t, uint64(0), d.AccountID())
}

func TestDriver_Info(t *testing.T) {
	d := makeDriver(t, makeSource("openrtb"), &mockHTTPClient{})
	info := d.Info()
	require.NotNil(t, info)
	assert.Equal(t, "42", info.ID)
	assert.Equal(t, "openrtb", info.Protocol)
}
