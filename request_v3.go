package adsourceopenrtb

import (
	"encoding/json"

	openrtbnreq "github.com/bsm/openrtb/native/request"
	"github.com/bsm/openrtb/v3"
	"github.com/geniusrabbit/udetect"

	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
)

func requestToRTBv3(req *adtype.BidRequest, opts ...BidRequestRTBOption) *openrtb.BidRequest {
	var opt BidRequestRTBOptions
	for _, fn := range opts {
		fn(&opt)
	}
	return &openrtb.BidRequest{
		ID:                req.ID,
		Impressions:       openrtbV3Impressions(req, &opt),
		Site:              uopenrtbOpenrtbV3SiteFrom(req.SiteInfo()),
		App:               uopenrtbOpenrtbV3ApplicationFrom(req.AppInfo()),
		Device:            uopenrtbOpenrtbV3DeviceFrom(req.DeviceInfo(), req.UserInfo().Geo),
		User:              uopenrtbOpenrtbV3UserInfo(req.UserInfo()),
		AuctionType:       int(opt.AuctionType),            // 1 = First Price, 2 = Second Price Plus
		TimeMax:           int(opt.TimeMax.Milliseconds()), // Maximum amount of time in milliseconds to submit a bid
		Seats:             nil,                             // Array of buyer seats allowed to bid on this auction
		AllImpressions:    0,                               //
		Currencies:        opt.currencies(),                // Array of allowed currencies
		BlockedCategories: nil,                             // Blocked Advertiser Categories
		BlockedAdvDomains: nil,                             // Array of strings of blocked toplevel domains of advertisers
		Regulations:       nil,
		Ext:               nil,
	}
}

func openrtbV3Impressions(req *adtype.BidRequest, opts *BidRequestRTBOptions) (list []openrtb.Impression) {
	for _, imp := range req.Imps {
		for _, format := range imp.Formats() {
			if openRTBImp := openrtbV3ImpressionByFormat(req, &imp, format, opts); openRTBImp != nil {
				list = append(list, *openRTBImp)
			}
		}
	}
	return list
}

func openrtbV3ImpressionByFormat(req *adtype.BidRequest, imp *adtype.Impression, format *types.Format, opts *BidRequestRTBOptions) *openrtb.Impression {
	var (
		banner *openrtb.Banner
		video  *openrtb.Video
		native *openrtb.Native
		ext    json.RawMessage
	)

	switch {
	case format.IsBanner() || format.IsProxy():
		w, h := imp.Width, imp.Height
		wm, wh := imp.WidthMax, imp.HeightMax
		if w < 1 && h < 1 {
			w, h = format.Width, format.Height
		}
		if !format.IsStretch() {
			wm, wh = 0, 0
		}
		banner = &openrtb.Banner{
			ID:           "",
			Width:        max(w, 5),
			Height:       max(h, 5),
			WidthMax:     wm,
			HeightMax:    wh,
			WidthMin:     0,
			HeightMin:    0,
			Position:     openrtb.AdPosition(imp.Pos),
			BlockedTypes: nil,
			BlockedAttrs: nil,
			MIMEs:        nil,
			TopFrame:     0,
			ExpDirs:      nil,
			APIs:         nil,
			Ext:          nil,
		}
	case format.IsNative():
		native = &openrtb.Native{
			Request:      openrtbV3NativeRequest(req, imp, format, opts),
			Version:      opts.openNativeVer(),
			APIs:         nil,
			BlockedAttrs: nil,
			Ext:          nil,
		}
	case format.IsDirect():
		ext = json.RawMessage(`{"type":"pop"}`)
	default:
		return nil
	}

	tagid := imp.Target.Codename() + "_" + format.Codename
	return &openrtb.Impression{
		ID:                    imp.IDByFormat(format),
		Banner:                banner,
		Video:                 video,
		Native:                native,
		DisplayManager:        "",                                            // Name of ad mediation partner, SDK technology, etc
		DisplayManagerVersion: "",                                            // Version of the above
		Interstitial:          b2i(imp.IsDirect()),                           // Interstitial, Default: 0 ("1": Interstitial, "0": Something else)
		TagID:                 tagid,                                         // IDentifier for specific ad placement or ad tag
		BidFloor:              max(imp.BidFloorCPM.Float64(), opts.BidFloor), // Bid floor for this impression in CPM
		BidFloorCurrency:      "",                                            // Currency of bid floor
		Secure:                openrtb.NumberOrString(b2i(req.IsSecure())),   // Flag to indicate whether the impression requires secure HTTPS URL creative assets and markup.
		IFrameBusters:         nil,                                           // Array of names for supportediframe busters.
		PMP:                   nil,                                           // A reference to the PMP object containing any Deals eligible for the impression object.
		Ext:                   ext,
	}
}

func openrtbV3NativeRequest(req *adtype.BidRequest, imp *adtype.Impression, format *types.Format, opts *BidRequestRTBOptions) json.RawMessage {
	native := &openrtbnreq.Request{
		Ver:              opts.openNativeVer(),                    // Version of the Native Markup
		LayoutID:         0,                                       // DEPRECATED The Layout ID of the native ad
		AdUnitID:         0,                                       // DEPRECATED The Ad unit ID of the native ad
		ContextTypeID:    imp.ContextType(),                       // The context in which the ad appears
		ContextSubTypeID: imp.ContextSubType(),                    // A more detailed context in which the ad appears
		PlacementTypeID:  imp.PlacementType(),                     // The design/format/layout of the ad unit being offered
		PlacementCount:   imp.Count,                               // The number of identical placements in this Layout
		Sequence:         0,                                       // 0 for the first ad, 1 for the second ad, and so on
		Assets:           openrtbV3NativeAssets(req, imp, format), // An array of Asset Objects
		Ext:              nil,
	}

	nativePrepared, _ := json.Marshal(native)

	// We have to encode it as a JSON string
	nativePrepared, _ = json.Marshal(`{"native":` + string(nativePrepared) + `}`)

	return json.RawMessage(nativePrepared)
}

func openrtbV3NativeAssets(_ *adtype.BidRequest, _ *adtype.Impression, format *types.Format) []openrtbnreq.Asset {
	assets := make([]openrtbnreq.Asset, 0, len(format.Config.Assets)+len(format.Config.Fields))
	for _, asset := range format.Config.Assets {
		if !asset.IsVideoSupport() || asset.IsImageSupport() {
			// By default we suppose that this is image
			var typeid openrtbnreq.ImageTypeID
			switch asset.Name {
			case types.FormatAssetMain:
				typeid = openrtbnreq.ImageTypeMain
			case types.FormatAssetIcon:
				typeid = openrtbnreq.ImageTypeIcon
			case "logo":
				typeid = openrtbnreq.ImageTypeLogo
			}
			assets = append(assets, openrtbnreq.Asset{
				ID:       int(asset.ID),
				Required: b2i(asset.Required),
				Image: &openrtbnreq.Image{
					TypeID:    typeid,
					WidthMin:  asset.MinWidth,
					HeightMin: asset.MinHeight,
					Mimes:     asset.AllowedTypes,
				},
			})
		}
		//  else {
		// 	// TODO add video tag support
		// }
	}
	for _, field := range format.Config.Fields {
		if asset, ok := openrtbV3NativeFieldAsset(&field); ok {
			assets = append(assets, asset)
		}
	}
	return assets
}

func uopenrtbOpenrtbV3UserInfo(u *adtype.User) *openrtb.User {
	data := make([]openrtb.Data, 0, len(u.Data))
	for _, it := range u.Data {
		dataItem := openrtb.Data{Name: it.Name}
		for i := 0; i < len(it.Segment); i++ {
			dataItem.Segment = append(dataItem.Segment, openrtb.Segment{
				Name:  it.Segment[i].Name,
				Value: it.Segment[i].Value,
			})
		}
		data = append(data, dataItem)
	}

	return &openrtb.User{
		ID:          u.ID,       // Unique consumer ID of this user on the exchange
		BuyerID:     "",         // Buyer-specific ID for the user as mapped by the exchange for the buyer. At least one of buyeruid/buyerid or id is recommended. Valid for OpenRTB 2.3.
		BuyerUID:    "",         // Buyer-specific ID for the user as mapped by the exchange for the buyer. Same as BuyerID but valid for OpenRTB 2.2.
		YearOfBirth: 0,          // Year of birth as a 4-digit integer.
		Gender:      u.Gender,   // Gender ("M": male, "F" female, "O" Other)
		Keywords:    u.Keywords, // Comma separated list of keywords, interests, or intent
		CustomData:  "",         // Optional feature to pass bidder data that was set in the exchange's cookie. The string must be in base85 cookie safe characters and be in any format. Proper JSON encoding must be used to include "escaped" quotation marks.
		Geo:         uopenrtbOpenrtbV3GeoFrom(u.Geo),
		Data:        data,
		Ext:         nil,
	}
}

func uopenrtbOpenrtbV3GeoFrom(g *udetect.Geo) *openrtb.Geo {
	return &openrtb.Geo{
		Latitude:      g.Lat,           // Latitude from -90 to 90
		Longitude:     g.Lon,           // Longitude from -180 to 180
		Type:          0,               // Indicate the source of the geo data
		Accuracy:      0,               // Estimated location accuracy in meters; recommended when lat/lon are specified and derived from a device’s location services
		LastFix:       0,               // Number of seconds since this geolocation fix was established.
		IPService:     0,               // Service or provider used to determine geolocation from IP address if applicable
		Country:       g.Country,       // Country using ISO 3166-1 Alpha 3
		Region:        g.Region,        // Region using ISO 3166-2
		RegionFIPS104: g.RegionFIPS104, // Region of a country using FIPS 10-4
		Metro:         g.Metro,         //
		City:          g.City,          //
		ZIP:           g.ZIP,           //
		UTCOffset:     g.UTCOffset,     // Local time as the number +/- of minutes from UTC
		Ext:           nil,             //
	}
}

func uopenrtbOpenrtbV3SiteFrom(s *udetect.Site) *openrtb.Site {
	if s == nil {
		return nil
	}
	cats := make([]openrtb.ContentCategory, 0, len(s.Cat))
	for _, ct := range s.Cat {
		cats = append(cats, openrtb.ContentCategory(ct))
	}
	return &openrtb.Site{
		Inventory: openrtb.Inventory{
			ID:            s.ExtID,                 // External ID
			Keywords:      s.Keywords,              // Comma separated list of keywords about the site.
			Categories:    cats,                    // Array of IAB content categories
			Domain:        s.Domain,                //
			PrivacyPolicy: intRef(s.PrivacyPolicy), // Default: 1 ("1": has a privacy policy)
		},
		Page:     s.Page,     // URL of the page
		Referrer: s.Referrer, // Referrer URL
		Search:   s.Search,   // Search string that caused naviation
		Mobile:   s.Mobile,   // Mobile ("1": site is mobile optimised)
	}
}

func uopenrtbOpenrtbV3ApplicationFrom(a *udetect.App) *openrtb.App {
	if a == nil {
		return nil
	}
	cats := make([]openrtb.ContentCategory, 0, len(a.Cat))
	for _, ct := range a.Cat {
		cats = append(cats, openrtb.ContentCategory(ct))
	}
	return &openrtb.App{
		Inventory: openrtb.Inventory{
			ID:            a.ExtID,                 // External ID
			Keywords:      a.Keywords,              // Comma separated list of keywords about the site.
			Categories:    cats,                    // Array of IAB content categories
			PrivacyPolicy: intRef(a.PrivacyPolicy), // Default: 1 ("1": has a privacy policy)
		},
		Bundle:   a.Bundle,   // App bundle or package name
		StoreURL: a.StoreURL, // App store URL for an installed app
		Version:  a.Ver,      // App version
		Paid:     a.Paid,     // "1": Paid, "2": Free
	}
}

func uopenrtbOpenrtbV3DeviceType(dt udetect.DeviceType) openrtb.DeviceType {
	switch dt {
	case udetect.DeviceTypeMobile:
		return openrtb.DeviceTypeMobile
	case udetect.DeviceTypePC:
		return openrtb.DeviceTypePC
	case udetect.DeviceTypeTV:
		return openrtb.DeviceTypeTV
	case udetect.DeviceTypePhone:
		return openrtb.DeviceTypePhone
	case udetect.DeviceTypeTablet:
		return openrtb.DeviceTypeTablet
	case udetect.DeviceTypeConnected:
		return openrtb.DeviceTypeConnected
	case udetect.DeviceTypeSetTopBox:
		return openrtb.DeviceTypeSetTopBox
	case udetect.DeviceTypeWatch, udetect.DeviceTypeGlasses:
	}
	return openrtb.DeviceTypeUnknown
}

func uopenrtbOpenrtbV3DeviceFrom(d *udetect.Device, geo *udetect.Geo) *openrtb.Device {
	if d == nil {
		return nil
	}
	var (
		browser = d.Browser
		os      = d.OS
		carrier *udetect.Carrier
		ipV4    = geo.IPv4String()
	)
	if browser == nil {
		browser = &udetect.BrowserDefault
	}
	if os == nil {
		os = &udetect.OSDefault
	}
	if geo == nil {
		geo = &udetect.GeoDefault
	}
	if carrier = geo.Carrier; carrier == nil {
		carrier = &udetect.CarrierDefault
	}

	// IP by default
	if ipV4 == "" && geo.IPv6String() == "" {
		ipV4 = "0.0.0.0"
	}

	return &openrtb.Device{
		UA:           browser.UA,                                // User agent
		Geo:          uopenrtbOpenrtbV3GeoFrom(geo),             // Location of the device assumed to be the user’s current location
		DNT:          browser.DNT,                               // "1": Do not track
		LMT:          browser.LMT,                               // "1": Limit Ad Tracking
		IP:           ipV4,                                      // IPv4
		IPv6:         geo.IPv6String(),                          // IPv6
		DeviceType:   uopenrtbOpenrtbV3DeviceType(d.DeviceType), // The general type of d.
		Make:         d.Make,                                    // Device make
		Model:        d.Model,                                   // Device model
		OS:           os.Name,                                   // Device OS
		OSVersion:    os.Version,                                // Device OS version
		HWVersion:    d.HwVer,                                   // Hardware version of the device (e.g., "5S" for iPhone 5S).
		Height:       d.Height,                                  // Physical height of the screen in pixels.
		Width:        d.Width,                                   // Physical width of the screen in pixels.
		PPI:          d.PPI,                                     // Screen size as pixels per linear inch.
		PixelRatio:   d.PxRatio,                                 // The ratio of physical pixels to device independent pixels.
		JS:           browser.JS,                                // Javascript status ("0": Disabled, "1": Enabled)
		GeoFetch:     0,                                         // Indicates if the geolocation API will be available to JavaScript code running in the banner,
		FlashVersion: browser.FlashVer,                          // Flash version
		Language:     browser.PrimaryLanguage,                   // Browser language
		Carrier:      carrier.Name,                              // Carrier or ISP derived from the IP address
		MCCMNC:       "",                                        // Mobile carrier as the concatenated MCC-MNC code (e.g., "310-005" identifies Verizon Wireless CDMA in the USA).
		ConnType:     openrtb.ConnType(d.ConnType),              // Network connection type.
		IFA:          d.IFA,                                     // Native identifier for advertisers
		IDSHA1:       "",                                        // SHA1 hashed device ID
		IDMD5:        "",                                        // MD5 hashed device ID
		PIDSHA1:      "",                                        // SHA1 hashed platform device ID
		PIDMD5:       "",                                        // MD5 hashed platform device ID
		MacSHA1:      "",                                        // SHA1 hashed device ID; IMEI when available, else MEID or ESN
		MacMD5:       "",                                        // MD5 hashed device ID; IMEI when available, else MEID or ESN
	}
}

func openrtbV3NativeFieldAsset(field *types.FormatField) (openrtbnreq.Asset, bool) {
	switch field.Name {
	case types.FormatFieldTitle:
		return openrtbnreq.Asset{
			ID:       field.ID,
			Required: b2i(field.Required),
			Title:    &openrtbnreq.Title{Length: field.MaxLength()},
		}, true
	case types.FormatFieldDescription:
		return openrtbnreq.Asset{
			ID:       field.ID,
			Required: b2i(field.Required),
			Data: &openrtbnreq.Data{
				TypeID: openrtbnreq.DataTypeDesc,
				Length: field.MaxLength(),
			},
		}, true
	case types.FormatFieldBrandname:
		return openrtbnreq.Asset{
			ID:       field.ID,
			Required: b2i(field.Required),
			Data: &openrtbnreq.Data{
				TypeID: openrtbnreq.DataTypeSponsored,
				Length: field.MaxLength(),
			},
		}, true
	case types.FormatFieldPhone:
		return openrtbnreq.Asset{
			ID:       field.ID,
			Required: b2i(field.Required),
			Data: &openrtbnreq.Data{
				TypeID: openrtbnreq.DataTypePhone,
				Length: field.MaxLength(),
			},
		}, true
	case types.FormatFieldURL:
		return openrtbnreq.Asset{
			ID:       field.ID,
			Required: b2i(field.Required),
			Data: &openrtbnreq.Data{
				TypeID: openrtbnreq.DataTypeDisplayURL,
				Length: field.MaxLength(),
			},
		}, true
	case types.FormatFieldRating:
		return openrtbnreq.Asset{
			ID:       field.ID,
			Required: b2i(field.Required),
			Data: &openrtbnreq.Data{
				TypeID: openrtbnreq.DataTypeRating,
				Length: field.MaxLength(),
			},
		}, true
	case types.FormatFieldLikes:
		return openrtbnreq.Asset{
			ID:       field.ID,
			Required: b2i(field.Required),
			Data: &openrtbnreq.Data{
				TypeID: openrtbnreq.DataTypeLikes,
				Length: field.MaxLength(),
			},
		}, true
	case types.FormatFieldAddress:
		return openrtbnreq.Asset{
			ID:       field.ID,
			Required: b2i(field.Required),
			Data: &openrtbnreq.Data{
				TypeID: openrtbnreq.DataTypeAddress,
				Length: field.MaxLength(),
			},
		}, true
	case types.FormatFieldSponsored:
		return openrtbnreq.Asset{
			ID:       field.ID,
			Required: b2i(field.Required),
			Data: &openrtbnreq.Data{
				TypeID: openrtbnreq.DataTypeSponsored,
				Length: field.MaxLength(),
			},
		}, true
	}
	return openrtbnreq.Asset{}, false
}
