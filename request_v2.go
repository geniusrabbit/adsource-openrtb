package adsourceopenrtb

import (
	"encoding/json"
	"fmt"

	"github.com/bsm/openrtb"
	openrtbnreq "github.com/bsm/openrtb/native/request"
	uopenrtb "github.com/geniusrabbit/udetect/openrtb"

	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
)

func requestToRTBv2(req *adtype.BidRequest, opts ...BidRequestRTBOption) *openrtb.BidRequest {
	var opt BidRequestRTBOptions
	for _, fn := range opts {
		fn(&opt)
	}
	return &openrtb.BidRequest{
		ID:          req.ID,
		Imp:         openrtbV2Impressions(req, &opt),
		Site:        uopenrtb.SiteFrom(req.SiteInfo()),
		App:         uopenrtb.ApplicationFrom(req.AppInfo()),
		Device:      uopenrtb.DeviceFrom(req.DeviceInfo(), req.UserInfo().Geo),
		User:        req.UserInfo().RTBObject(),
		AuctionType: int(opt.AuctionType),            // 1 = First Price, 2 = Second Price Plus
		TMax:        int(opt.TimeMax.Milliseconds()), // Maximum amount of time in milliseconds to submit a bid
		WSeat:       nil,                             // Array of buyer seats allowed to bid on this auction
		AllImps:     0,                               //
		Cur:         opt.currencies(),                // Array of allowed currencies
		Bcat:        nil,                             // Blocked Advertiser Categories
		BAdv:        nil,                             // Array of strings of blocked toplevel domains of advertisers
		Regs:        nil,
		Ext:         nil,
	}
}

func openrtbV2Impressions(req *adtype.BidRequest, opts *BidRequestRTBOptions) (list []openrtb.Impression) {
	for _, imp := range req.Imps {
		for _, format := range imp.Formats() {
			if openRTBImp := openrtbV2ImpressionByFormat(req, &imp, format, opts); openRTBImp != nil {
				list = append(list, *openRTBImp)
			}
		}
	}
	return list
}

func openrtbV2ImpressionByFormat(req *adtype.BidRequest, imp *adtype.Impression, format *types.Format, opts *BidRequestRTBOptions) *openrtb.Impression {
	var (
		banner *openrtb.Banner
		video  *openrtb.Video
		native *openrtb.Native
		ext    openrtb.Extension
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
			ID:       "",
			W:        max(w, 5),
			H:        max(h, 5),
			WMax:     wm,
			HMax:     wh,
			WMin:     0,
			HMin:     0,
			Pos:      imp.Pos,
			BType:    nil,
			BAttr:    nil,
			Mimes:    nil,
			TopFrame: 0,
			ExpDir:   nil,
			Api:      nil,
			Ext:      nil,
		}
	case format.IsNative():
		native = &openrtb.Native{
			Request: openrtbV2NativeRequest(req, imp, format, opts),
			Ver:     opts.openNativeVer(),
			API:     nil,
			BAttr:   nil,
			Ext:     nil,
		}
	case format.IsDirect():
		ext = openrtb.Extension(`{"type":"pop"}`)
	default:
		return nil
	}

	tagid := imp.Target.Codename() + "_" + format.Codename
	return &openrtb.Impression{
		ID:                imp.IDByFormat(format),
		Banner:            banner,
		Video:             video,
		Native:            native,
		DisplayManager:    "",                                          // Name of ad mediation partner, SDK technology, etc
		DisplayManagerVer: "",                                          // Version of the above
		Instl:             b2i(imp.IsDirect()),                         // Interstitial, Default: 0 ("1": Interstitial, "0": Something else)
		TagID:             tagid,                                       // IDentifier for specific ad placement or ad tag
		BidFloor:          max(imp.BidFloor.Float64(), opts.BidFloor),  // Bid floor for this impression in CPM
		BidFloorCurrency:  "",                                          // Currency of bid floor
		Secure:            openrtb.NumberOrString(b2i(req.IsSecure())), // Flag to indicate whether the impression requires secure HTTPS URL creative assets and markup.
		IFrameBuster:      nil,                                         // Array of names for supportediframe busters.
		Pmp:               nil,                                         // A reference to the PMP object containing any Deals eligible for the impression object.
		Ext:               ext,
	}
}

func openrtbV2NativeRequest(req *adtype.BidRequest, imp *adtype.Impression, format *types.Format, opts *BidRequestRTBOptions) openrtb.Extension {
	var (
		nativePrepared []byte
		native         *openrtbnreq.Request
	)

	if native = imp.RTBNativeRequest(); native == nil {
		native = &openrtbnreq.Request{
			Ver:              opts.openNativeVer(),                    // Version of the Native Markup
			LayoutID:         0,                                       // DEPRECATED The Layout ID of the native ad
			AdUnitID:         0,                                       // DEPRECATED The Ad unit ID of the native ad
			ContextTypeID:    imp.ContextType(),                       // The context in which the ad appears
			ContextSubTypeID: imp.ContextSubType(),                    // A more detailed context in which the ad appears
			PlacementTypeID:  imp.PlacementType(),                     // The design/format/layout of the ad unit being offered
			PlacementCount:   imp.Count,                               // The number of identical placements in this Layout
			Sequence:         0,                                       // 0 for the first ad, 1 for the second ad, and so on
			Assets:           openrtbV2NativeAssets(req, imp, format), // An array of Asset Objects
			Ext:              nil,
		}
	}

	nativePrepared, _ = json.Marshal(native)

	// We have to encode it as a JSON string
	nativePrepared, _ = json.Marshal(`{"native":` + string(nativePrepared) + `}`)

	return openrtb.Extension(nativePrepared)
}

func openrtbV2NativeAssets(_ *adtype.BidRequest, _ *adtype.Impression, format *types.Format) []openrtbnreq.Asset {
	assets := make([]openrtbnreq.Asset, 0, len(format.Config.Assets)+len(format.Config.Fields))
	fmt.Println("> openrtbV3NativeAssets", format.Config)
	for _, asset := range format.Config.Assets {
		fmt.Println("> LOG ASSET", asset)
		if !asset.IsVideoSupport() || asset.IsImageSupport() {
			// By default we suppose that this is image
			var typeid openrtbnreq.ImageTypeID
			switch asset.Name {
			case types.FormatAssetMain:
				typeid = openrtbnreq.ImageTypeMain
			case types.FormatAssetIcon:
				typeid = openrtbnreq.ImageTypeIcon
			case types.FormatAssetLogo:
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
		// TODO add video tag support
	}
	for _, field := range format.Config.Fields {
		if asset, ok := openrtbV2NativeFieldAsset(&field); ok {
			assets = append(assets, asset)
		}
	}
	return assets
}

func openrtbV2NativeFieldAsset(field *types.FormatField) (openrtbnreq.Asset, bool) {
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
