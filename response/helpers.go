//
// @project GeniusRabbit corelib 2017 - 2019, 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019, 2025
//

package response

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"net/url"
	"strings"

	"golang.org/x/net/html/charset"
)

// decodePopMarkup extracts the popunder URL from an XML ad markup payload.
// It supports two common XML structures: <popunderAd><url> and <ad><popunderAd><url>.
//
//lint:ignore U1000 unused function
func decodePopMarkup(data []byte) (val string, err error) {
	var item struct {
		URL1 string `xml:"popunderAd>url"`
		URL2 string `xml:"ad>popunderAd>url"`
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = charset.NewReaderLabel
	if err = decoder.Decode(&item); err == nil {
		if val = item.URL1; val == "" {
			val = item.URL2
		}
	}
	return val, err
}

// customDirectURL extracts a direct landing page URL from a JSON ad markup payload.
// It checks the "url", "landingpage", and "link" fields in order.
//
//lint:ignore U1000 unused function
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

// prepareURL decodes percent-encoding from a URL string and applies the given
// macro [strings.Replacer], returning the processed URL.
// Returns an empty string if surl is empty.
func prepareURL(surl string, replacer *strings.Replacer) string {
	if surl == "" {
		return surl
	}
	if u, err := url.QueryUnescape(surl); err == nil {
		surl = u
	}
	return replacer.Replace(surl)
}

// openNativeVASTTagInfo holds a subset of VAST 2.0 fields used for native ad
// format detection (e.g., determining if a markup is a video ad tag).
//
// Example VAST 2.0 wrapper:
//
//	<VAST version='2.0'>
//	  <Ad id='12345'>
//	    <InLine>
//	      <AdSystem>AdServer</AdSystem>
//	      <AdTitle>Test Ad</AdTitle>
//	      <Creatives>
//	        <Creative>
//	          <Linear>
//	            <Duration>00:00:30</Duration>
//	            <MediaFiles>
//	              <MediaFile delivery='progressive' type='video/mp4' width='640' height='360'>
//	                <![CDATA[http://example.com/vast_tag.mp4]]>
//	              </MediaFile>
//	            </MediaFiles>
//	            <VideoClicks>
//	              <ClickThrough><![CDATA[http://example.com/click_here]]></ClickThrough>
//	            </VideoClicks>
//	          </Linear>
//	        </Creative>
//	      </Creatives>
//	    </InLine>
//	  </Ad>
//	</VAST>
type openNativeVASTTagInfo struct {
	Version string `xml:"version,attr"`
	Ad      struct {
		ID     string `xml:"id,attr"`
		InLine struct {
			AdSystem  string `xml:"AdSystem"`
			AdTitle   string `xml:"AdTitle"`
			Creatives struct {
				Creative []struct {
					Linear struct {
						Duration   string `xml:"Duration"`
						MediaFiles struct {
							MediaFile []struct {
								Delivery string `xml:"delivery,attr"`
								Type     string `xml:"type,attr"`
								Width    int    `xml:"width,attr"`
								Height   int    `xml:"height,attr"`
								URL      string `xml:",chardata"`
							} `xml:"MediaFile"`
						} `xml:"MediaFiles"`
						VideoClicks struct {
							ClickThrough string `xml:"ClickThrough"`
						} `xml:"VideoClicks"`
					} `xml:"Linear"`
				} `xml:"Creative"`
			} `xml:"Creatives"`
		} `xml:"InLine"`
	} `xml:"Ad"`
}

// parseOpenNativeVASTtag decodes a VAST 2.0 XML payload into [openNativeVASTTagInfo].
// The charset-aware decoder is used to handle non-UTF-8 encoded documents.
//
//lint:ignore U1000 unused function
func parseOpenNativeVASTtag(data []byte) (*openNativeVASTTagInfo, error) {
	var item openNativeVASTTagInfo
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(&item); err != nil {
		return nil, err
	}
	return &item, nil
}
