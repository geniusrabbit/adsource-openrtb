# OpenRTB Driver
  
  [![Go Report Card](https://goreportcard.com/badge/github.com/geniusrabbit/adsource-openrtb)](https://goreportcard.com/report/github.com/geniusrabbit/adsource-openrtb)
  [![Go Reference](https://pkg.go.dev/badge/github.com/geniusrabbit/adsource-openrtb.svg)](https://pkg.go.dev/github.com/geniusrabbit/adsource-openrtb)
  [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

## Overview

The **OpenRTB Driver** is a robust Go package designed to facilitate interaction with the [OpenRTB](https://www.iab.com/guidelines/real-time-bidding-openrtb-project/) (Real-Time Bidding) protocol. It enables seamless handling of real-time bidding requests and responses in compliance with OpenRTB standards (versions 2.x and 3.x).

## Features

- **Request and Response Handling**: Supports bid requests and responses for both OpenRTB 2.x and 3.x protocols.
- **Metrics and Logging**: Comprehensive metrics collection and logging integrated with [Zap](https://github.com/uber-go/zap) and [Prometheus](https://prometheus.io/) using `prometheuswrapper`.
- **Error Handling**: Implements robust error handling with retry mechanisms.
- **Customizable Headers**: Allows customization of HTTP request headers to suit specific requirements.
- **Rate Limiting**: Supports Requests Per Second (RPS) limits to control the rate of outgoing requests.
- **Extensible Architecture**: Easily extendable to accommodate additional functionalities or custom behaviors.

## Installation

Ensure you have Go installed (version 1.18 or higher). Then, you can install the OpenRTB Driver package using `go get`:

```bash
go get github.com/geniusrabbit/adsource-openrtb
```

## Usage

### Initialization

To initialize the OpenRTB driver, you need to set up the context, source details, and HTTP client.

```go
import (
    "context"
    "github.com/geniusrabbit/adcorelib/admodels"
    "github.com/geniusrabbit/adcorelib/net/httpclient"
    "github.com/geniusrabbit/adsourceopenrtb"
)

func main() {
    ctx := context.Background()
    source := &admodels.RTBSource{
        // Initialize with source details
        ID:        12345,
        Protocol:  "openrtb2",
        URL:       "https://example.com/bid",
        RPS:       100,
        Timeout:   500, // in milliseconds
        Headers:   admodels.Headers{Data: map[string]string{"Authorization": "Bearer token"}},
        Options:   admodels.Options{Trace: 1, ErrorsIgnore: 1},
        // Additional fields...
    }
    netClient := httpclient.New() // Or your custom HTTP client

    driver, err := adsourceopenrtb.NewDriver(ctx, source, netClient)
    if err != nil {
        // Handle initialization error
        panic(err)
    }

    // Use the driver...
}
```

### Sending a Bid Request

Once the driver is initialized, you can send bid requests as follows:

```go
import (
    "github.com/geniusrabbit/adcorelib/adtype"
)

func sendBid(driver *adsourceopenrtb.Driver, ctx context.Context) {
    request := &adtype.BidRequest{
        // Initialize bid request fields
    }

    if driver.Test(request) {
        response := driver.Bid(request)
        if response.Error() != nil {
            // Handle bid error
        } else {
            // Process successful bid response
            for _, ad := range response.Ads() {
                // Handle each ad
            }
        }
    }
}
```

### Handling Metrics

The driver provides metrics that can be accessed and processed as needed.

```go
metrics := driver.Metrics()
// Example: Log the metrics
log.Printf("Metrics: %+v", metrics)
```

## API Overview

### `driver` Struct

The `driver` struct is the core component managing the lifecycle of bid requests, including preparation, execution, and response processing.

#### Key Methods

- **`NewDriver`**: Initializes a new driver instance.
  
  ```go
  func NewDriver[ND httpclient.Driver[Rq, Rs], Rq httpclient.Request, Rs httpclient.Response](
      ctx context.Context,
      source *admodels.RTBSource,
      netClient ND,
      options ...any,
  ) (*driver[ND, Rq, Rs], error)
  ```

- **`ID`**: Returns the ID of the source.
  
  ```go
  func (d *driver) ID() uint64
  ```

- **`Protocol`**: Returns the protocol version.
  
  ```go
  func (d *driver) Protocol() string
  ```

- **`Test`**: Tests if the request meets the criteria for processing.
  
  ```go
  func (d *driver) Test(request *adtype.BidRequest) bool
  ```

- **`PriceCorrectionReduceFactor`**: Returns the price correction reduce factor.
  
  ```go
  func (d *driver) PriceCorrectionReduceFactor() float64
  ```

- **`RequestStrategy`**: Returns the request strategy.
  
  ```go
  func (d *driver) RequestStrategy() adtype.RequestStrategy
  ```

- **`Bid`**: Processes a bid request and returns a response.
  
  ```go
  func (d *driver) Bid(request *adtype.BidRequest) adtype.Responser
  ```

- **`ProcessResponseItem`**: Processes individual response items.
  
  ```go
  func (d *driver) ProcessResponseItem(response adtype.Responser, item adtype.ResponserItem)
  ```

- **`RevenueShareReduceFactor`**: Returns the revenue share reduce factor.
  
  ```go
  func (d *driver) RevenueShareReduceFactor() float64
  ```

- **`Metrics`**: Returns platform metrics.
  
  ```go
  func (d *driver) Metrics() *openlatency.MetricsInfo
  ```

### Interfaces

- **`adtype.SourceMinimal`**: Minimal set of methods required for a source.

  ```go
  type SourceMinimal interface {
      Bid(request *BidRequest) Responser
      ProcessResponseItem(Responser, ResponserItem)
  }
  ```

- **`adtype.Source`**: Extends `SourceMinimal` with additional methods.

  ```go
  type Source interface {
      SourceMinimal
      ID() uint64
      ObjectKey() uint64
      Protocol() string
      Test(request *BidRequest) bool
      PriceCorrectionReduceFactor() float64
      RequestStrategy() RequestStrategy
  }
  ```

- **`adtype.SourceTesteChecker`**: Interface for testing request compatibility.

  ```go
  type SourceTesteChecker interface {
      Test(request *BidRequest) bool
  }
  ```

- **`adtype.SourceTimeoutSetter`**: Interface to set timeouts for the source.

  ```go
  type SourceTimeoutSetter interface {
      SetTimeout(timeout time.Duration)
  }
  ```

- **`adtype.SourceTester`**: Combines `Source` and `SourceTesteChecker`.

  ```go
  type SourceTester interface {
      Source
      SourceTesteChecker
  }
  ```

## Error Handling

The driver implements robust error handling mechanisms. Errors encountered during bid requests are logged, and appropriate metrics are updated. It supports retry mechanisms and ensures that the system remains resilient under failure conditions.

## Logging and Metrics

Logging is integrated using the [Zap](https://github.com/uber-go/zap) library, providing high-performance structured logging. Metrics are collected and exposed using [Prometheus](https://prometheus.io/) via the `prometheuswrapper`, enabling real-time monitoring and alerting.

## Customization

The driver allows customization of HTTP request headers and supports rate limiting through configurable RPS (Requests Per Second) settings. This ensures that the driver can be tailored to meet specific application requirements and adhere to external API constraints.

## Contributing

Contributions are welcome! Please follow these steps:

1. Fork the repository.
2. Create a new branch for your feature or bugfix.
3. Commit your changes with clear and descriptive messages.
4. Open a pull request detailing your changes.

For major changes, please open an issue first to discuss what you would like to change.

## License

[LICENSE](LICENSE)

Copyright 2024 Dmitry Ponomarev & Geniusrabbit

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

<http://www.apache.org/licenses/LICENSE-2.0>

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.
