package v3

import (
	"encoding/json"

	"github.com/bsm/openrtb/v3"
)

type ImpressionWrapper openrtb.Impression

func (i *ImpressionWrapper) SetExt(ext map[string]any) {
	data, _ := json.Marshal(ext)
	i.Ext = data
}
