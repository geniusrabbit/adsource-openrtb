//
// @project GeniusRabbit corelib 2017 - 2019, 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019, 2025
//

// Package interstitial provides XML markup parsing utilities for interstitial ad formats.
// It is an internal package shared between the banner and direct response subpackages
// to avoid circular imports.
package interstitial

import (
	"encoding/xml"
	"strings"

	"golang.org/x/net/html/charset"
)

// ParseAdMarkup parses XML ad markup for interstitial ad formats and extracts
// the relevant URL components. It supports the following ad types embedded in the XML:
//   - iframeAd: returns the iframe URL as pageURL
//   - popunderAd: returns the popunder URL as pageURL
//   - imageAd: returns clickURL and imgURL
//
// Example iframeAd markup:
//
//	<?xml version="1.0" encoding="ISO-8859-1"?>
//	<ad>
//	  <iframeAd>
//	    <url><![CDATA[https://example.com/iframe]]></url>
//	  </iframeAd>
//	</ad>
//
// Example imageAd markup:
//
//	<?xml version="1.0" encoding="ISO-8859-1"?>
//	<ad>
//	  <imageAd>
//	    <clickUrl><![CDATA[https://example.com/redirect]]></clickUrl>
//	    <imgUrl><![CDATA[https://i.imgur.com/bRoshBm_d.webp]]></imgUrl>
//	  </imageAd>
//	</ad>
//
// Example popunderAd markup:
//
//	<?xml version="1.0" encoding="ISO-8859-1"?>
//	<ad>
//	  <popunderAd>
//	    <url><![CDATA[https://example.com/popunder]]></url>
//	  </popunderAd>
//	</ad>
func ParseAdMarkup(adMarkup string) (pageURL, clickURL, imgURL string, err error) {
	type cdataString struct {
		CDATA string `xml:",cdata"`
	}
	var item struct {
		XMLName     xml.Name    `xml:"ad"`
		PopunderURL cdataString `xml:"popunderAd>url"`
		IFrameURL   cdataString `xml:"iframeAd>url"`
		ClickURL    cdataString `xml:"imageAd>clickUrl"`
		ImgURL      cdataString `xml:"imageAd>imgUrl"`
	}
	decoder := xml.NewDecoder(strings.NewReader(adMarkup))
	decoder.CharsetReader = charset.NewReaderLabel
	if err = decoder.Decode(&item); err != nil {
		return "", "", "", err
	}
	if item.IFrameURL.CDATA != "" {
		return item.IFrameURL.CDATA, "", "", nil
	}
	if item.PopunderURL.CDATA != "" {
		return item.PopunderURL.CDATA, "", "", nil
	}
	return "", item.ClickURL.CDATA, item.ImgURL.CDATA, nil
}
