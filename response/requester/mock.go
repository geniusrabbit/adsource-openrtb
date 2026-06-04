package requester

import "github.com/geniusrabbit/adcorelib/adtype"

// MockRTBRequester is a test implementation of RTBRequester that returns
// a pre-configured response without making any real HTTP calls.
type MockRTBRequester struct {
	// Response to return on DoServerRequest. If nil, ErrResponseNoBid is returned.
	Response adtype.Response
	// Err to return on DoServerRequest. Overrides Response when non-nil.
	Err error
}

// Request returns the pre-configured Response and Err fields.
func (m *MockRTBRequester) Request(_ adtype.BidRequester, _ uint64) (adtype.Response, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Response, nil
}
