package requester

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	openrtb "github.com/bsm/openrtb"

	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"

	"github.com/geniusrabbit/adsource-openrtb/response"
)

type MatchItem struct {
	BidResponse *openrtb.BidResponse
	Imp         *adtype.Impression
	AdFormat    *types.Format
}

// BuildResponse returns a deep-copied, ID-patched BidResponse that contains only
// the bids compatible with m.Imp (as determined by bidMatchesImpression).
// Returns an error when no qualifying bids survive the filter.
func (m *MatchItem) BuildResponse() (*openrtb.BidResponse, error) {
	copied := *m.BidResponse
	copied.ID = adtype.NewRequestID()

	seats := make([]openrtb.SeatBid, 0, len(m.BidResponse.SeatBid))
	for _, seat := range m.BidResponse.SeatBid {
		var bids []openrtb.Bid
		for _, bid := range seat.Bid {
			if bidMatchesImpression(bid, m.Imp) != nil {
				b := bid
				b.ID = adtype.NewAdResponseItemID()
				b.ImpID = m.Imp.IDByFormat(m.AdFormat)
				bids = append(bids, b)
			}
		}
		if len(bids) > 0 {
			patched := seat
			patched.Seat = adtype.NewAdResponseItemID()
			patched.Bid = bids
			seats = append(seats, patched)
		}
	}

	if len(seats) == 0 {
		return nil, fmt.Errorf("match item: no compatible bids for impression %s", m.Imp.ID)
	}

	copied.SeatBid = seats
	return &copied, nil
}

// SimulationRTBRequester implements RTBRequester for simulation and replay testing.
// At construction it accepts a list of raw JSON OpenRTB bid-response strings and
// converts them into template objects. On each Request call it:
//   - filters templates by ad-type compatibility and minimum bid floor
//   - selects randomly from the matching set (falls back to all templates if none match)
//   - deep-copies the selected template
//   - generates fresh IDs for BidResponse.ID and every Bid.ID
//   - assigns a new ID to every SeatBid.Seat
//   - rewires every Bid.ImpID to cycle through the impressions present in the request
type SimulationRTBRequester struct {
	src       adtype.Source
	templates []openrtb.BidResponse
}

// NewSimulationRTBRequester parses jsonResponses into OpenRTB response objects
// and returns a ready-to-use SimulationRTBRequester.
// Returns an error if any JSON is malformed or the slice is empty.
func NewSimulationRTBRequester(jsonResponses []string) (*SimulationRTBRequester, error) {
	if len(jsonResponses) == 0 {
		return nil, fmt.Errorf("simulation requester: at least one JSON response is required")
	}
	templates := make([]openrtb.BidResponse, 0, len(jsonResponses))
	for i, js := range jsonResponses {
		var resp openrtb.BidResponse
		if err := json.Unmarshal([]byte(js), &resp); err != nil {
			return nil, fmt.Errorf("simulation requester: response[%d]: %w", i, err)
		}
		templates = append(templates, resp)
	}
	return &SimulationRTBRequester{templates: templates}, nil
}

// SetSource sets the adtype.Source used to tag bid response items.
// Call this after the driver (which implements adtype.Source) has been created.
func (s *SimulationRTBRequester) SetSource(src adtype.Source) {
	s.src = src
}

// Request selects the next template response, deep-copies it, patches
// all identifiers (BidResponse.ID, Bid.ID, SeatBid.Seat, Bid.ImpID) and
// returns a fully prepared adtype.Response.
func (s *SimulationRTBRequester) Request(request adtype.BidRequester, _ uint64) (adtype.Response, error) {
	imps := request.Impressions()
	if len(imps) == 0 {
		return nil, nil
	}

	// Filter templates by ad type and bid floor, then pick randomly.
	matching := s.matchingTemplates(imps)
	if len(matching) == 0 {
		return nil, nil
	}
	item := matching[rand.Intn(len(matching))]
	if item == nil || len(item.BidResponse.SeatBid) == 0 {
		return nil, nil
	}
	res, err := item.BuildResponse()
	if err != nil {
		return nil, err
	}

	bidResp := &response.BidResponse{
		Src:         s.src,
		Req:         request,
		BidResponse: *res,
	}
	bidResp.Prepare()
	return bidResp, nil
}

// matchingTemplates returns all (template, impression, format) triples where the
// template has at least one bid meeting the bid floor and compatible with the impression.
func (s *SimulationRTBRequester) matchingTemplates(imps []*adtype.Impression) []*MatchItem {
	var maxBidFloor float64
	for _, imp := range imps {
		if f := imp.BidFloorCPM.Float64(); f > maxBidFloor {
			maxBidFloor = f
		}
	}

	matching := make([]*MatchItem, 0, len(s.templates))
	for i := range s.templates {
		tmpl := &s.templates[i]
		for _, imp := range imps {
			if format := bidResponseMatchesImpression(tmpl, imp, maxBidFloor); format != nil {
				matching = append(matching, &MatchItem{
					BidResponse: tmpl,
					Imp:         imp,
					AdFormat:    format,
				})
			}
		}
	}
	return matching
}

// bidResponseMatchesImpression returns the first matching format when the template
// contains at least one bid that meets minPrice and is compatible with imp.
func bidResponseMatchesImpression(tmpl *openrtb.BidResponse, imp *adtype.Impression, minPrice float64) *types.Format {
	for _, seat := range tmpl.SeatBid {
		for _, bid := range seat.Bid {
			if bid.Price < minPrice {
				continue
			}
			if f := bidMatchesImpression(bid, imp); f != nil {
				return f
			}
		}
	}
	return nil
}

// bidMatchesImpression returns the matched format for a bid against a single impression.
// Checks ".adformat" ext key first, then falls back to markup heuristic.
func bidMatchesImpression(bid openrtb.Bid, imp *adtype.Impression) *types.Format {
	if codes := bidExtFormatCodes(bid.Ext); len(codes) > 0 {
		for _, code := range codes {
			if f := imp.FormatByCode(code); f != nil {
				return f
			}
		}
		return nil
	}
	return markupFormatForImpression(bid.AdMarkup, imp)
}

// bidExtFormatCodes extracts the ".adformat" string slice from bid.Ext, e.g.:
//
//	{"ext": {".adformat": ["banner_300x250"]}}
//
// Returns nil when the key is absent or Ext cannot be parsed.
func bidExtFormatCodes(ext openrtb.Extension) []string {
	if len(ext) == 0 {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(ext, &m); err != nil {
		return nil
	}
	raw, ok := m[".adformat"]
	if !ok {
		return nil
	}
	var codes []string
	if err := json.Unmarshal(raw, &codes); err != nil {
		return nil
	}
	return codes
}

// markupFormatForImpression detects the creative type from markup content and returns
// the matching format from the impression, or nil if incompatible.
func markupFormatForImpression(markup string, imp *adtype.Impression) *types.Format {
	m := strings.TrimLeft(markup, " \t\r\n")
	if len(m) == 0 {
		return nil
	}
	switch {
	case strings.HasPrefix(m, "<VAST") || strings.HasPrefix(m, "<vast") || strings.HasPrefix(m, "<?xml"):
		return imp.FormatByType(types.FormatVideoType)
	case m[0] == '{':
		return imp.FormatByType(types.FormatNativeType)
	case strings.HasPrefix(m, "http://") || strings.HasPrefix(m, "https://"):
		return imp.FormatByType(types.FormatDirectType)
	default:
		// Assume HTML banner markup
		if f := imp.FormatByType(types.FormatBannerType); f != nil {
			return f
		}
		return imp.FormatByType(types.FormatBannerHTML5Type)
	}
}
