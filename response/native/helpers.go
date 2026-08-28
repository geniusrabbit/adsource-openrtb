//
// @project GeniusRabbit corelib 2017 - 2019, 2025
// @author Dmitry Ponomarev <demdxx@gmail.com> 2017 - 2019, 2025
//

package native

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime"
	"path/filepath"

	"github.com/bsm/openrtb/native/request"
	natresp "github.com/bsm/openrtb/native/response"
	requestV3 "github.com/bsm/openrtb/v3/native/request"

	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/models"
)

// decodeNativeMarkup decodes a raw JSON native ad markup into a [response.Response].
// It handles two common wire formats:
//  1. A top-level wrapper object: {"native": {...}}
//  2. The native response object directly: {"link": ..., "assets": [...]}
func decodeNativeMarkup(nativeResp *natresp.Response, data []byte) (err error) {
	var (
		native = struct {
			Native *natresp.Response `json:"native"`
		}{
			Native: nativeResp,
		}
	)
	if bytes.Contains(data, []byte(`"native"`)) {
		if err = json.Unmarshal(data, &native); err != nil {
			err = json.Unmarshal(data, native.Native)
		}
	} else {
		err = json.Unmarshal(data, native.Native)
	}
	return err
}

// openrtbNativeLabelNameByType maps an OpenRTB Native data asset type ID to its
// canonical field name used in the ad content map.
func openrtbNativeLabelNameByType(dataTypeID int) string {
	switch request.DataTypeID(dataTypeID) {
	case request.DataTypeSponsored:
		return models.FormatFieldBrandname
	case request.DataTypeDesc:
		return models.FormatFieldDescription
	case request.DataTypeRating:
		return models.FormatFieldRating
	case request.DataTypeLikes:
		return models.FormatFieldLikes
	// case request.DataTypeDownloads:
	// 	return models.FormatFieldDownloads
	// case request.DataTypePrice:
	// 	return models.FormatFieldPrice
	// case request.DataTypeSalePrice:
	// 	return models.FormatFieldSalePrice
	case request.DataTypePhone:
		return models.FormatFieldPhone
	case request.DataTypeAddress:
		return models.FormatFieldAddress
	// case request.DataTypeDescAdditional:
	// 	return models.FormatFieldDescAdditional
	case request.DataTypeDisplayURL:
		return models.FormatFieldURL
		// case request.DataTypeCTADesc:
		// 	return models.FormatFieldCTADesc
	}
	return ""
}

// extractNativeV2Data extracts native ad data from an OpenRTB Native v1.x/v2.x
// request and response pair. Asset IDs in the response are matched against the
// request to determine the correct field name for each data asset.
func extractNativeV2Data(req *request.Request, resp *natresp.Response) map[string]any {
	data := map[string]any{}
	data[adtype.ContentItemLink] = resp.Link.URL

	for _, asset := range resp.Assets {
		if asset.Title != nil {
			data[types.FormatFieldTitle] = asset.Title.Text
		} else if asset.Data != nil {
			for _, ass := range req.Assets {
				if ass.ID == asset.ID && ass.Data != nil {
					name := openrtbNativeLabelNameByType(int(ass.Data.TypeID))
					if name == "" && asset.Data.Label != "" {
						name = asset.Data.Label
					}
					if name != "" {
						data[name] = asset.Data.Value
					}
					break
				}
			}
		}
	}
	return data
}

// extractNativeV3Data extracts native ad data from an OpenRTB Native v3.x request
// paired with a v1.x/v2.x response. Asset IDs are matched across protocol versions
// to determine the correct field name for each data asset.
func extractNativeV3Data(req *requestV3.Request, resp *natresp.Response) map[string]any {
	data := map[string]any{}
	data[adtype.ContentItemLink] = resp.Link.URL

	for _, asset := range resp.Assets {
		if asset.Title != nil {
			data[types.FormatFieldTitle] = asset.Title.Text
		} else if asset.Data != nil {
			for _, ass := range req.Assets {
				if ass.ID == asset.ID && ass.Data != nil {
					name := openrtbNativeLabelNameByType(int(ass.Data.TypeID))
					if name == "" && asset.Data.Label != "" {
						name = asset.Data.Label
					}
					if name != "" {
						data[name] = asset.Data.Value
					}
					break
				}
			}
		}
	}
	return data
}

// extractNativeDataFromImpression selects the appropriate extract function based on
// whether the impression carries an OpenRTB Native v2.x or v3.x request, and returns
// the resulting field map. Returns nil if no native request is attached.
//
//go:inline
func extractNativeDataFromImpression(imp *adtype.Impression, native *natresp.Response) map[string]any {
	if nativeRequestV2 := imp.RTBNativeRequest(); nativeRequestV2 != nil {
		return extractNativeV2Data(nativeRequestV2, native)
	} else if nativeRequestV3 := imp.RTBNativeRequestV3(); nativeRequestV3 != nil {
		return extractNativeV3Data(nativeRequestV3, native)
	}
	return nil
}

// validateRequiredAssets checks that every required image/video asset declared in
// the format config is present (by matching asset ID) in the native response.
// Returns [ErrMissingRequiredAsset] on the first missing required asset.
func validateRequiredAssets(format *types.Format, native *natresp.Response) error {
	if format == nil || format.Config == nil {
		return nil
	}
	for _, configAsset := range format.Config.Assets {
		if !configAsset.IsRequired() {
			continue
		}
		found := false
		for _, asset := range native.Assets {
			if asset.ID == configAsset.ID && (asset.Image != nil || asset.Video != nil) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: asset id=%d name=%q", ErrMissingRequiredAsset, configAsset.ID, configAsset.GetName())
		}
	}
	return nil
}

// extractContentTypeFromFileName returns the MIME content type based on the file extension of the given filename.
// It uses the standard library's mime.TypeByExtension function to determine the content type.
// If the extension is not recognized, it returns an empty string.
func extractContentTypeFromFileName(filename string) string {
	return mime.TypeByExtension(filepath.Ext(filename))
}
