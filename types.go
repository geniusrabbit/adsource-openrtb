package adsourceopenrtb

import (
	"github.com/geniusrabbit/adcorelib/admodels"

	rtbreq "github.com/geniusrabbit/adsource-openrtb/response/requester"
)

// Request type enum
const (
	RequestTypeUndefined       = admodels.RTBRequestTypeUndefined
	RequestTypeJSON            = admodels.RTBRequestTypeJSON
	RequestTypeXML             = admodels.RTBRequestTypeXML
	RequestTypeProtobuff       = admodels.RTBRequestTypeProtoBUFF
	RequestTypePOSTFormEncoded = admodels.RTBRequestTypePOSTFormEncoded
	RequestTypePlain           = admodels.RTBRequestTypePLAINTEXT
)

// Re-export sentinel errors from the requester subpackage so callers that
// import this top-level package retain the same error identity.
var (
	ErrResponseAreNotSecure   = rtbreq.ErrResponseAreNotSecure
	ErrInvalidResponseStatus  = rtbreq.ErrInvalidResponseStatus
	ErrResponseNoBid          = rtbreq.ErrResponseNoBid
	ErrUnsupportedRequestType = rtbreq.ErrUnsupportedRequestType
	ErrUndefinedRequestType   = rtbreq.ErrUndefinedRequestType
	ErrNilSource              = rtbreq.ErrNilSource
	ErrNilHTTPClient          = rtbreq.ErrNilHTTPClient
)

// Re-export error types as aliases to maintain backward compatibility.
type (
	HTTPStatusError      = rtbreq.HTTPStatusError
	UnsupportedTypeError = rtbreq.UnsupportedTypeError
	UndefinedTypeError   = rtbreq.UndefinedTypeError
)
