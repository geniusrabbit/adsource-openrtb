// Package requester defines the RTBRequester interface and its implementations
// for executing RTB source round-trips.
package requester

import "github.com/geniusrabbit/adcorelib/adtype"

// RTBRequester wraps the execution of a single RTB server round-trip.
// Implementations are responsible for sending a bid request to an RTB source
// and returning the resulting adtype.Response.
type RTBRequester interface {
	Request(request adtype.BidRequester, beginTime uint64) (adtype.Response, error)
}
