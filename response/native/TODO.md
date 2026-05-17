# OpenRTB Coverage Gaps — Native

This file tracks OpenRTB fields and features that are **not yet handled** in the native response item.

## Native Response Object

| Field                    | Description                                            | Current status |
| ------------------------ | ------------------------------------------------------ | -------------- |
| `native.jstracker`       | JavaScript impression tracker string                   | Not used       |
| `native.privacy`         | Privacy icon URL                                       | Not exposed    |
| `native.ver`             | OpenRTB Native spec version of the response            | Not validated  |
| `native.eventtrackers[]` | Structured event tracker objects (OpenRTB Native 1.2+) | Not used       |

## Asset — Image

| Feature                          | Description                        | Current status                                                 |
| -------------------------------- | ---------------------------------- | -------------------------------------------------------------- |
| `asset.img.type` differentiation | `1`=Icon, `2`=Logo, `3`=Main image | All stored as `AdFileAssetImageType` — no sub-type distinction |
| `asset.img.mimes[]`              | Acceptable MIME types              | Not used                                                       |
| `asset.img.ext`                  | Vendor extensions                  | Not used                                                       |

## Asset — Data

| Feature                      | Description         | Current status                           |
| ---------------------------- | ------------------- | ---------------------------------------- |
| `data.type` = 3 (Rating)     | Star rating         | Label-based lookup; `data.type` not used |
| `data.type` = 4 (Likes)      | Like count          | Same                                     |
| `data.type` = 6 (Downloads)  | Download count      | Same                                     |
| `data.type` = 10 (Price)     | Product price       | Same                                     |
| `data.type` = 11 (SalePrice) | Sale price          | Same                                     |
| `data.type` = 12 (Phone)     | Phone number        | Same                                     |
| `data.type` = 14 (CTA text)  | Call-to-action text | Same                                     |

## Asset — Video

| Feature                        | Description               | Current status       |
| ------------------------------ | ------------------------- | -------------------- |
| Inline video VAST tag playback | Only `VASTTag` URL stored | VAST XML not decoded |

## Trackers

| Feature                          | Description                         | Current status |
| -------------------------------- | ----------------------------------- | -------------- |
| Per-asset `link.clicktrackers[]` | Click trackers per individual asset | Not gathered   |
| `native.jstracker`               | JS viewability/impression pixel     | Not injected   |

## OpenRTB 3.x Native

| Feature                     | Description           | Current status |
| --------------------------- | --------------------- | -------------- |
| OpenRTB 3.x native response | Different asset model | Not supported  |

## Validation

| Feature                            | Description                      | Current status                          |
| ---------------------------------- | -------------------------------- | --------------------------------------- |
| `asset.required=1` in bid response | Required flag echoed back by DSP | Not cross-checked against format config |
