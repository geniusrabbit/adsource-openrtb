package adsourceopenrtb

import (
	"errors"

	"github.com/geniusrabbit/adcorelib/admodels"
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

// Errors set
var (
	ErrResponseAreNotSecure  = errors.New("response are not secure")
	ErrInvalidResponseStatus = errors.New("invalid response status")
	ErrNoCampaignsStatus     = errors.New("no campaigns response")
)
