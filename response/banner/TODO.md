# OpenRTB Coverage Gaps — Banner

This file tracks OpenRTB fields and features that are **not yet handled** in the banner response item.

## Bid Object (`openrtb.Bid`)

| Field                  | Description                                             | Current status                                                |
| ---------------------- | ------------------------------------------------------- | ------------------------------------------------------------- |
| `bid.Exp`              | Impression expiry in seconds                            | Not used                                                      |
| `bid.DealID`           | Deal ID for PMP transactions                            | Not used                                                      |
| `bid.Attr`             | Creative attributes (expandable, audio, etc.)           | Not used                                                      |
| `bid.Lang`             | Language of the creative                                | Not used                                                      |
| `bid.Bundle`           | App bundle of the advertiser                            | Not used                                                      |
| `bid.IURL`             | Campaign image URL for quality scanning                 | Not used                                                      |
| `bid.NURL`             | Win-notice URL (price macro substitution)               | URL returned but **`${AUCTION_PRICE}` macro not substituted** |
| `bid.BURL`             | Billing-notice URL (same macro issue)                   | Same                                                          |
| `bid.CID` / `bid.CrID` | Campaign / creative ID (available but never propagated) | Partially exposed via `CreativeID()`                          |

## Banner Creative

| Feature              | Description                                      | Current status                |
| -------------------- | ------------------------------------------------ | ----------------------------- |
| `banner.format[]`    | Multi-size responsive banner                     | Only `bid.W` / `bid.H` used   |
| Expandable banners   | `bid.Attr` values 2–4, 9–10                      | Not detected                  |
| MRAID creatives      | Mobile rich-media                                | No detection / wrapping       |
| Secure markup check  | HTTP URLs in `bid.AdMarkup` for HTTPS placements | Not validated                 |
| Image-only creatives | `bid.AdMarkup` = image URL without a click URL   | Falls through to `HTML` field |
| `bid.AdSlot`         | Ad slot identifier                               | Not used                      |

## Tracking

| Feature                       | Description                                 | Current status |
| ----------------------------- | ------------------------------------------- | -------------- |
| Click trackers from `bid.Ext` | Some DSPs pass click trackers in extensions | Not parsed     |
| Viewability scripts           | `bid.Ext` viewability vendor pixels         | Not parsed     |

## Interstitial XML (MRAID / Fulls-Screen)

| Feature                                    | Description                               | Current status |
| ------------------------------------------ | ----------------------------------------- | -------------- |
| Multiple `<Creatives>` in interstitial XML | Only first creative used                  | Not handled    |
| Video URL in interstitial                  | `<Video>` element inside interstitial XML | Not parsed     |
