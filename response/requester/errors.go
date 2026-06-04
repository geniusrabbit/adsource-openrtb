package requester

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/geniusrabbit/adcorelib/adtype"
)

// Errors set — sentinel errors for type-assertable error handling.
var (
	// ErrResponseAreNotSecure is returned when an HTTP response is received
	// for a request that requires HTTPS (i.e. contains http:// in markup).
	ErrResponseAreNotSecure = errors.New("response are not secure")

	// ErrInvalidResponseStatus is returned when the HTTP response status code
	// is not one of: 200, 204, 404.
	ErrInvalidResponseStatus = errors.New("invalid response status")

	// ErrResponseNoBid is returned when the RTB source explicitly has no bid
	// (HTTP 204 / 404 or empty SeatBid list).
	ErrResponseNoBid = adtype.ErrResponseNoBid

	// ErrUnsupportedRequestType is returned when the source is configured with
	// a request body encoding that is not yet implemented (e.g. XML, Protobuf).
	ErrUnsupportedRequestType = errors.New("request body type not supported")

	// ErrUndefinedRequestType is returned when the source has an unrecognised
	// RTBRequestType value.
	ErrUndefinedRequestType = errors.New("undefined request type")

	// ErrNilSource is returned when the RTBSource pointer is nil.
	ErrNilSource = errors.New("rtb source must not be nil")

	// ErrNilHTTPClient is returned when the HTTP client is nil.
	ErrNilHTTPClient = errors.New("http client must not be nil")
)

// HTTPStatusError is a typed error that carries the unexpected HTTP status code
// returned by an RTB endpoint.  It wraps ErrInvalidResponseStatus so callers
// can use errors.Is(err, ErrInvalidResponseStatus) while still being able to
// inspect the concrete code via errors.As.
type HTTPStatusError struct {
	Code int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("invalid response status: %d %s", e.Code, http.StatusText(e.Code))
}

// Is satisfies errors.Is for the sentinel ErrInvalidResponseStatus.
func (e *HTTPStatusError) Is(target error) bool {
	return target == ErrInvalidResponseStatus
}

// UnsupportedTypeError is returned when the source is configured with a request
// body encoding that is not implemented.
type UnsupportedTypeError struct {
	TypeName string
}

func (e *UnsupportedTypeError) Error() string {
	return fmt.Sprintf("request body type not supported: %s", e.TypeName)
}

// Is satisfies errors.Is for the sentinel ErrUnsupportedRequestType.
func (e *UnsupportedTypeError) Is(target error) bool {
	return target == ErrUnsupportedRequestType
}

// UndefinedTypeError is returned when the source has an unrecognised
// RTBRequestType value.
type UndefinedTypeError struct {
	TypeName string
}

func (e *UndefinedTypeError) Error() string {
	return fmt.Sprintf("undefined request type: %s", e.TypeName)
}

// Is satisfies errors.Is for the sentinel ErrUndefinedRequestType.
func (e *UndefinedTypeError) Is(target error) bool {
	return target == ErrUndefinedRequestType
}
