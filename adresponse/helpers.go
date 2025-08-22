package adresponse

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"net/url"
	"strings"

	"golang.org/x/net/html/charset"

	"github.com/geniusrabbit/adcorelib/admodels/types"
)

func decodePopMarkup(data []byte) (val string, err error) {
	var item struct {
		URL string `xml:"popunderAd>url"`
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = charset.NewReaderLabel
	if err = decoder.Decode(&item); err == nil {
		val = item.URL
	}
	return val, err
}

func customDirectURL(data []byte) (val string, err error) {
	var item struct {
		URL         string `json:"url"`
		LandingPage string `json:"landingpage"`
		Link        string `json:"link"`
	}
	if err = json.Unmarshal(data, &item); err == nil {
		val = max(item.URL, item.LandingPage, item.Link)
	}
	return val, err
}

func bannerFormatType(markup string) types.FormatType {
	if strings.HasPrefix(markup, "http://") ||
		strings.HasPrefix(markup, "https://") ||
		(strings.HasPrefix(markup, "//") && !strings.ContainsAny(markup, "\n\t")) ||
		strings.Contains(markup, "<iframe") {
		return types.FormatProxyType
	}
	return types.FormatBannerType
}

func prepareURL(surl string, replacer *strings.Replacer) string {
	if surl == "" {
		return surl
	}
	if u, err := url.QueryUnescape(surl); err == nil {
		surl = u
	}
	return replacer.Replace(surl)
}
