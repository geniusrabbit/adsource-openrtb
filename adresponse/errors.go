package adresponse

import "errors"

var (
	ErrInvalidAdContent             = errors.New("invalid ad content")
	ErrInvalidVAST                  = errors.New("invalid VAST response")
	ErrUnsupportedVASTConfiguration = errors.New("unsupported VAST configuration")
)
