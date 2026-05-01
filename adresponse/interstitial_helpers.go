package adresponse

import (
	"encoding/xml"
	"strings"

	"golang.org/x/net/html/charset"
)

// parseInterstitialAdMarkup parses the ad markup for interstitial ads and extracts the iframe URL or HTML content.
// It handles both cases where the ad markup is a URL (starting with "http://" or "https://") and where it is an XML string (starting with "<?xml").
// For XML ad markup, it extracts the click URL and image URL from the XML structure and constructs the appropriate HTML content for the interstitial ad.
//
// Example of XML ad markup:
//
//	<?xml version="1.0" encoding="ISO-8859-1"?>
//	<ad>
//		<imageAd>
//			<clickUrl><![CDATA[https://example.com/redirect]]></clickUrl>
//			<imgUrl><![CDATA[https://i.imgur.com/bRoshBm_d.webp]]></imgUrl>
//		</imageAd>
//	</ad>
//
// Example of XML ad markup with Popunder URL:
//
//	<?xml version="1.0" encoding="ISO-8859-1"?>
//	<ad>
//		<popunderAd>
//			<url><![CDATA[https://example.com/popunder]]></url>
//		</popunderAd>
//	</ad>
//
// Example of XML ad markup with IFrame URL:
//
//	<?xml version="1.0" encoding="ISO-8859-1"?>
//	<ad>
//		<iframeAd>
//			<url><![CDATA[https://example.com/iframe]]></url>
//		</iframeAd>
//	</ad>
//
// TODO: Add support for VAST XML ad markup if needed in the future.
//
// Example of videoAd markup with VAST XML:
//
//	<?xml version="1.0" encoding="ISO-8859-1"?>
//	<ad>
//		<videoAd>
//			<vast><![CDATA[
//				<VAST version="2.0">
//					<Ad id="12345">
//						<InLine>
//							<Creatives>
//								<Creative>
//									<Linear>
//										<MediaFiles>
//											<MediaFile><![CDATA[https://example.com/video.mp4]]></MediaFile>
//										</MediaFiles>
//									</Linear>
//								</Creative>
//							</Creatives>
//						</InLine>
//					</Ad>
//				</VAST>
//			]]></vast>
//		</videoAd>
//	</ad>
func parseInterstitialAdMarkup(adMarkup string) (pageURL, clickURL, imgURL string, err error) {
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
