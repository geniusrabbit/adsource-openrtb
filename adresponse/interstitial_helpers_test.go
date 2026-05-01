package adresponse

import "testing"

func TestParseInterstitialAdMarkup(t *testing.T) {
	tests := []struct {
		name      string
		adMarkup  string
		wantURL   string
		wantClick string
		wantImg   string
		wantErr   bool
	}{
		{
			name: "Valid IFrame URL",
			adMarkup: `<?xml version="1.0" encoding="ISO-8859-1"?>` +
				`<ad>` +
				`	<iframeAd>` +
				`		<url><![CDATA[https://example.com/iframe]]></url>` +
				`	</iframeAd>` +
				`</ad>`,
			wantURL:   "https://example.com/iframe",
			wantClick: "",
			wantImg:   "",
			wantErr:   false,
		},
		{
			name: "Valid Image Ad",
			adMarkup: `<?xml version="1.0" encoding="ISO-8859-1"?>` +
				`<ad>` +
				`	<imageAd>` +
				`		<clickUrl><![CDATA[https://example.com/redirect]]></clickUrl>` +
				`		<imgUrl><![CDATA[https://i.imgur.com/bRoshBm_d.webp]]></imgUrl>` +
				`	</imageAd>` +
				`</ad>`,
			wantURL:   "",
			wantClick: "https://example.com/redirect",
			wantImg:   "https://i.imgur.com/bRoshBm_d.webp",
			wantErr:   false,
		},
		{
			name: "Valid Popunder URL",
			adMarkup: `<?xml version="1.0" encoding="ISO-8859-1"?>` +
				`<ad>` +
				`	<popunderAd>` +
				`		<url><![CDATA[https://example.com/popunder]]></url>` +
				`	</popunderAd>` +
				`</ad>`,
			wantURL:   "https://example.com/popunder",
			wantClick: "",
			wantImg:   "",
			wantErr:   false,
		},
		{
			name: "Invalid XML",
			adMarkup: `<?xml version="1.0" encoding="ISO-8859-1"?>` +
				`<ad>` +
				`	<iframeAd>` +
				`		<url><![CDATA[https://example.com/iframe]]></url>` +
				`	</iframeAd>` +
				`<!-- Missing closing tag for ad -->`,
			wantURL:   "",
			wantClick: "",
			wantImg:   "",
			wantErr:   true,
		},
		{
			name:      "Empty Ad Markup",
			adMarkup:  `<?xml version="1.0" encoding="ISO-8859-1"?><ad></ad>`,
			wantURL:   "",
			wantClick: "",
			wantImg:   "",
			wantErr:   false,
		},
		{
			name:      "Custom",
			adMarkup:  "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><ad><popunderAd><url><![CDATA[https://pxl-eu.rtb.tsyndicate.com/do2/direct]]></url></popunderAd></ad>",
			wantURL:   "https://pxl-eu.rtb.tsyndicate.com/do2/direct",
			wantClick: "",
			wantImg:   "",
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotClick, gotImg, err := parseInterstitialAdMarkup(tt.adMarkup)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInterstitialAdMarkup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotURL != tt.wantURL {
				t.Errorf("parseInterstitialAdMarkup() gotURL = %v, want %v", gotURL, tt.wantURL)
			}
			if gotClick != tt.wantClick {
				t.Errorf("parseInterstitialAdMarkup() gotClick = %v, want %v", gotClick, tt.wantClick)
			}
			if gotImg != tt.wantImg {
				t.Errorf("parseInterstitialAdMarkup() gotImg = %v, want %v", gotImg, tt.wantImg)
			}
		})
	}
}
