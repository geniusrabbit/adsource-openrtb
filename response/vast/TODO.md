# OpenRTB Coverage Gaps — VAST

This file tracks OpenRTB fields and features that are **not yet handled** in the VAST response item.

## VAST Structure

| Feature                      | Description                               | Current status                 |
| ---------------------------- | ----------------------------------------- | ------------------------------ |
| Multiple `<Ad>` elements     | Ad Pods, competitive exclusion            | Only `Ads[0]` used             |
| `<Ad sequence>` attribute    | Ad pod sequence ordering                  | Not used                       |
| Multiple `<Creative>` per Ad | Multi-creative ads                        | Only `Creatives[0]` used       |
| `<Companion>` banners        | Companion display banners alongside video | Not extracted                  |
| `<NonLinear>` ads            | Overlay / non-linear creatives            | Not extracted                  |
| Wrapper chain depth > 1      | Nested VAST wrappers                      | Only one wrapper level handled |

## Tracking Events

| Feature                                                                    | Description             | Current status |
| -------------------------------------------------------------------------- | ----------------------- | -------------- |
| Quartile events (`firstQuartile`, `midpoint`, `thirdQuartile`, `complete`) | Video playback tracking | Not extracted  |
| Custom tracking events (`<Tracking event="...">`)                          | General custom events   | Not extracted  |
| `<Error>` URL                                                              | Error notification URL  | Not stored     |

## VAST 3+ Features

| Feature                         | Description                             | Current status |
| ------------------------------- | --------------------------------------- | -------------- |
| `<Pricing>` element (VAST 3.0)  | Declared price from DSP                 | Not used       |
| `<AdVerifications>` (VAST 4.0)  | Third-party viewability / verification  | Not extracted  |
| `<UniversalAdId>` (VAST 4.0)    | Standardised creative ID across systems | Not used       |
| VAST 4.x `<CreativeExtensions>` | Custom data extensions                  | Not parsed     |

## Bid Object (`openrtb.Bid`)

| Field                   | Description                               | Current status                               |
| ----------------------- | ----------------------------------------- | -------------------------------------------- |
| `bid.Protocol`          | VAST protocol version supported (1–8)     | Not validated against returned VAST version  |
| `bid.API[]`             | API frameworks (VPAID, OMID)              | Not detected                                 |
| `bid.Attr`              | Creative attributes (autoplay, skippable) | Not used                                     |
| `bid.NURL` / `bid.BURL` | Win/billing notice macros                 | **`${AUCTION_PRICE}` macro not substituted** |
| `bid.Dur`               | Duration of the video in seconds          | Not validated against creative               |

## Video Creatives

| Feature                     | Description                         | Current status                             |
| --------------------------- | ----------------------------------- | ------------------------------------------ |
| VPAID units                 | `<MediaFile apiFramework="VPAID">`  | Not distinguished from regular media files |
| `skipoffset`                | Skippable after N seconds           | Not extracted                              |
| Multiple bitrate renditions | Multiple `<MediaFile>` per creative | Stored as separate assets (OK)             |
