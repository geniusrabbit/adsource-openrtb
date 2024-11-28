package adresponse

import (
	"bytes"
	"encoding/json"

	"github.com/bsm/openrtb/native/request"
	"github.com/bsm/openrtb/native/response"
	requestV3 "github.com/bsm/openrtb/v3/native/request"
	"github.com/geniusrabbit/adcorelib/admodels/types"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/models"
)

func decodeNativeMarkup(data []byte) (*response.Response, error) {
	var (
		native struct {
			Native response.Response `json:"native"`
		}
		err error
	)
	if bytes.Contains(data, []byte(`"native"`)) {
		err = json.Unmarshal(data, &native)
	} else {
		err = json.Unmarshal(data, &native.Native)
	}
	if err != nil {
		err = json.Unmarshal(data, &native.Native)
	}
	if err != nil {
		return nil, err
	}
	return &native.Native, nil
}

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

func extractNativeV2Data(req *request.Request, resp *response.Response) map[string]any {
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

func extractNativeV3Data(req *requestV3.Request, resp *response.Response) map[string]any {
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

//go:inline
func extractNativeDataFromImpression(imp *adtype.Impression, native *response.Response) map[string]any {
	if nativeRequestV2 := imp.RTBNativeRequest(); nativeRequestV2 != nil {
		return extractNativeV2Data(nativeRequestV2, native)
	} else if nativeRequestV3 := imp.RTBNativeRequestV3(); nativeRequestV3 != nil {
		return extractNativeV3Data(nativeRequestV3, native)
	}
	return nil
}
