# adsource-openrtb

[![Build Status](https://github.com/geniusrabbit/adsource-openrtb/workflows/Tests/badge.svg)](https://github.com/geniusrabbit/adsource-openrtb/actions?workflow=Tests)
[![Go Report Card](https://goreportcard.com/badge/github.com/geniusrabbit/adsource-openrtb)](https://goreportcard.com/report/github.com/geniusrabbit/adsource-openrtb)
[![Go Reference](https://pkg.go.dev/badge/github.com/geniusrabbit/adsource-openrtb.svg)](https://pkg.go.dev/github.com/geniusrabbit/adsource-openrtb)
[![Coverage Status](https://coveralls.io/repos/github/geniusrabbit/adsource-openrtb/badge.svg)](https://coveralls.io/github/geniusrabbit/adsource-openrtb)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

OpenRTB demand-source driver for the [adcorelib](https://github.com/geniusrabbit/adcorelib) ad stack.
It handles the full lifecycle of an outbound bid: builds an OpenRTB request, fires it over HTTP,
parses the response, and maps each winning creative into a typed `adtype.ResponseItem` ready for auction.

## Protocol support

| Version                       | Status |
| ----------------------------- | ------ |
| OpenRTB 2.3 / 2.4 / 2.5 / 2.6 | ✅     |
| OpenRTB 3.0+                  | ✅     |

## Ad formats

Each format lives in its own sub-package under `response/` and implements `adtype.ResponseItem`:

| Package           | Format                         | Notes                                                      |
| ----------------- | ------------------------------ | ---------------------------------------------------------- |
| `response/banner` | Banner (HTML / iframe / image) | Parses interstitial XML markup                             |
| `response/direct` | Direct / pop (click URL)       | URL or interstitial XML                                    |
| `response/native` | Native                         | Decodes OpenRTB Native 1.x JSON; validates required assets |
| `response/vast`   | VAST video                     | Decodes VAST XML; extracts media files, trackers, icons    |

Shared identity, pricing, and auction logic lives in `response/common.BaseBidItem` and is promoted
into every format-specific struct via embedding. Format types that carry media assets (`native`, `vast`)
own their `assets` slice directly — `banner` and `direct` items have zero asset overhead.

## Installation

```bash
go get github.com/geniusrabbit/adsource-openrtb
```

Requires Go 1.21+.

## Quick start

```go
import (
    "context"
    "time"

    adsourceopenrtb "github.com/geniusrabbit/adsource-openrtb"
    "github.com/geniusrabbit/adcorelib/admodels"
    "github.com/geniusrabbit/adcorelib/net/httpclient"
)

// Build a factory that creates one HTTP client per RTB source.
factory := adsourceopenrtb.NewFactory(func(ctx context.Context, timeout time.Duration) (httpclient.Driver, error) {
    return httpclient.New(timeout), nil
})

// Create a driver for a single RTB endpoint.
source := &admodels.RTBSource{
    ID:      12345,
    URL:     "https://dsp.example.com/bid",
    RPS:     200,
    Timeout: 150, // ms
}
driver, err := factory.New(ctx, source)
if err != nil {
    log.Fatal(err)
}

// Send a bid request.
resp := driver.Bid(bidRequest)
if resp.Error() != nil {
    log.Println("no fill:", resp.Error())
    return
}
for _, item := range resp.Ads() {
    // item implements adtype.ResponseItem — pass to auction / renderer
}
```

## Package layout

```
adsource-openrtb/
├── driver.go              # Core driver: Bid(), Test(), ProcessResponseItem()
├── init.go                # Factory / registration helpers
├── types.go               # Shared type aliases
├── request/
│   ├── v2/                # OpenRTB 2.x request builder
│   ├── v3/                # OpenRTB 3.x request builder
│   └── options/           # Per-request option overrides
└── response/
    ├── common/            # BaseBidItem — shared fields & methods
    ├── banner/            # Banner response item
    ├── direct/            # Direct/pop response item
    ├── native/            # Native response item
    └── vast/              # VAST video response item
```

## Key driver methods

| Method                            | Description                                            |
| --------------------------------- | ------------------------------------------------------ |
| `Bid(request)`                    | Sends the bid request; returns `adtype.Responser`      |
| `Test(request)`                   | Returns true when the source can serve this request    |
| `ProcessResponseItem(resp, item)` | Win/billing notification callback                      |
| `PriceCorrectionReduceFactor()`   | Source-level price correction factor                   |
| `Metrics()`                       | Latency and error counters (`openlatency.MetricsInfo`) |

## Contributing

1. Fork the repository.
2. Create a feature branch.
3. Commit changes with clear messages.
4. Open a pull request.

For significant changes please open an issue first.

## License

[Apache 2.0](LICENSE) — Copyright 2017–2025 Dmitry Ponomarev & Geniusrabbit
