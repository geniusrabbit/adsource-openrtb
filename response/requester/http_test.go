package requester

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/adquery/bidrequest"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/net/httpclient"
)

// ─── minimal mock infrastructure ─────────────────────────────────────────────

type httpTestRequest struct {
	headers map[string]string
}

func (r *httpTestRequest) SetHeader(key, value string) {
	if r.headers == nil {
		r.headers = make(map[string]string)
	}
	r.headers[key] = value
}

type httpTestResponse struct {
	statusCode int
	body       io.Reader
}

func (r *httpTestResponse) StatusCode() int { return r.statusCode }
func (r *httpTestResponse) Body() io.Reader { return r.body }
func (r *httpTestResponse) Close() error    { return nil }

type httpTestClient struct {
	doFn func(req httpclient.Request) (httpclient.Response, error)
}

func (c *httpTestClient) Request(_, _ string, _ io.Reader) (httpclient.Request, error) {
	return &httpTestRequest{}, nil
}

func (c *httpTestClient) Do(req httpclient.Request) (httpclient.Response, error) {
	if c.doFn != nil {
		return c.doFn(req)
	}
	return &httpTestResponse{statusCode: http.StatusNoContent}, nil
}

// makeRTBSource returns a minimal RTBSource suitable for constructing an HttpRTBRequester.
func makeRTBSource(protocol string) *admodels.RTBSource {
	return &admodels.RTBSource{
		ID:          42,
		Protocol:    protocol,
		URL:         "http://example.com/rtb",
		Method:      "POST",
		RequestType: admodels.RTBRequestTypeJSON,
	}
}

// makeTestBidRequest returns a minimal adtype.BidRequester.
func makeTestBidRequest() *bidrequest.BidRequest {
	return &bidrequest.BidRequest{
		IDVal: "test-bid-id",
		Ctx:   context.Background(),
		Imps:  []*adtype.Impression{{ID: "imp-1"}},
	}
}

// ─── HttpRTBRequester.unmarshal ───────────────────────────────────────────────

func TestHttpRTBRequester_Unmarshal_UnsupportedXML(t *testing.T) {
	src := makeRTBSource("openrtb")
	src.RequestType = admodels.RTBRequestTypeXML
	r, err := NewHttpRTBRequester(src, &httpTestClient{})
	require.NoError(t, err)
	_, err = r.unmarshal(makeTestBidRequest(), strings.NewReader(""))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsupportedRequestType)
}

// ─── HttpRTBRequester.fillRequest – headers ───────────────────────────────────

func TestHttpRTBRequester_FillRequest_SetsOpenRTBHeader(t *testing.T) {
	r, err := NewHttpRTBRequester(makeRTBSource("openrtb"), &httpTestClient{})
	require.NoError(t, err)
	req := &httpTestRequest{}
	r.fillRequest(makeTestBidRequest(), req)
	assert.Equal(t, headerRequestOpenRTBVersion2, req.headers[headerRequestOpenRTBVersion])
}

func TestHttpRTBRequester_FillRequest_SetsOpenRTBv3Header(t *testing.T) {
	r, err := NewHttpRTBRequester(makeRTBSource("openrtb3"), &httpTestClient{})
	require.NoError(t, err)
	req := &httpTestRequest{}
	r.fillRequest(makeTestBidRequest(), req)
	assert.Equal(t, headerRequestOpenRTBVersion3, req.headers[headerRequestOpenRTBVersion])
}

func TestHttpRTBRequester_FillRequest_ContentType(t *testing.T) {
	r, err := NewHttpRTBRequester(makeRTBSource("openrtb"), &httpTestClient{})
	require.NoError(t, err)
	req := &httpTestRequest{}
	r.fillRequest(makeTestBidRequest(), req)
	assert.Equal(t, "application/json", req.headers["Content-Type"])
}
