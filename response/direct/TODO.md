# OpenRTB Coverage Gaps — Direct

This file tracks OpenRTB fields and features that are **not yet handled** in the direct response item.

## Bid Object (`openrtb.Bid`)

| Field                               | Description                               | Current status                               |
| ----------------------------------- | ----------------------------------------- | -------------------------------------------- |
| `bid.NURL`                          | Win-notice URL (price macro substitution) | **`${AUCTION_PRICE}` macro not substituted** |
| `bid.BURL`                          | Billing-notice URL                        | Same macro issue                             |
| `bid.AdID` / `bid.CID` / `bid.CrID` | Ad / campaign / creative identifiers      | Not used                                     |
| `bid.Attr`                          | Creative attributes                       | Not used                                     |
| `bid.Bundle`                        | Advertiser app bundle                     | Not used                                     |

## URL Handling

| Feature                             | Description                                               | Current status                      |
| ----------------------------------- | --------------------------------------------------------- | ----------------------------------- |
| Deep links (`custom-scheme://`)     | App deep-link URLs from mobile DSPs                       | Rejected — only `http/https//` pass |
| URL macro substitution              | `${AUCTION_PRICE}`, `${AUCTION_ID}`, etc. in `DirectLink` | Not performed                       |
| Fallback when `DirectLink` is empty | No fallback rendering                                     | Returns empty `ActionURL()`         |

## Content Rendering

| Feature                   | Description                                          | Current status                   |
| ------------------------- | ---------------------------------------------------- | -------------------------------- |
| JavaScript snippet        | Some DSPs return JS redirect snippets                | Not detected; treated as invalid |
| Iframe sandbox attributes | Current hardcoded `sandbox` may block some creatives | No per-bid override              |
