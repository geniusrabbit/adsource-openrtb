// Package openrtb provides implementation of the OpenRTB protocol for the adsource package.
// Supported versions: 2.3, 2.4, 2.5, 2.6, 3.0+
//
// The openrtb package is designed to facilitate interaction with the OpenRTB protocol, allowing for real-time bidding
// requests and responses within the adsource framework. It supports multiple versions of the protocol, ensuring compatibility
// with various OpenRTB standards.
//
// Key Features:
// - **Version Support**: Supports OpenRTB versions 2.3, 2.4, 2.5, 2.6, and 3.0+.
// - **Customizable Clients**: Allows for the creation of customizable HTTP clients through the NewClientFnk function.
// - **Timeout Handling**: Manages request timeouts with a default value and configurable options based on source settings.
// - **Platform Information**: Provides detailed platform information, including supported protocols and documentation links.
//
// Usage:
//
// Initialization of the Factory:
//   ctx := context.Background()
//   newClient := func(ctx context.Context, timeout time.Duration) (httpclient.Driver, error) {
//       return httpclient.New(timeout), nil // Implement your HTTP client creation logic
//   }
//
//   factory := openrtb.NewFactory(newClient)
//
// Creating a New Driver:
//   source := &admodels.RTBSource{ /* initialize with source details */ }
//   driver, err := factory.New(ctx, source)
//   if err != nil {
//       // handle error
//   }
//
// Retrieving Platform Information:
//   platformInfo := factory.Info()
//
// The main components of the package include the factory for creating new driver instances, customizable HTTP client function,
// and platform information retrieval.

package adsourceopenrtb

import (
	"context"
	"time"

	"github.com/demdxx/gocast/v2"
	"github.com/geniusrabbit/adcorelib/admodels"
	"github.com/geniusrabbit/adcorelib/adtype"
	"github.com/geniusrabbit/adcorelib/net/httpclient"
	"github.com/geniusrabbit/adcorelib/platform/info"
)

const (
	protocol       = "openrtb"
	defaultTimeout = 150 * time.Millisecond
)

type NewClientFnk func(context.Context, time.Duration) (httpclient.Driver, error)

type factory struct {
	newClientFnk NewClientFnk
}

func NewFactory(newClient NewClientFnk) *factory {
	return &factory{
		newClientFnk: newClient,
	}
}

func (fc *factory) New(ctx context.Context, source *admodels.RTBSource, opts ...any) (adtype.SourceTester, error) {
	ncli, err := fc.newClientFnk(ctx, gocast.IfThen(
		source.Timeout > 0,
		time.Duration(source.Timeout)*time.Millisecond,
		defaultTimeout,
	))
	if err != nil {
		return nil, err
	}
	dr, err := newDriver(ctx, source, ncli, opts...)
	if err != nil {
		return nil, err
	}
	return dr, nil
}

func (*factory) Info() info.Platform {
	return info.Platform{
		Name:        "OpenRTB",
		Protocol:    protocol,
		Versions:    []string{"2.3", "2.4", "2.5", "2.6", "3.0"},
		Description: "",
		Docs: []info.Documentation{
			{
				Title: "OpenRTB (Real-Time Bidding)",
				Link:  "https://www.iab.com/guidelines/real-time-bidding-rtb-project/",
			},
		},
		Subprotocols: []info.Subprotocol{
			{
				Name:     "VAST",
				Protocol: "vast",
				Docs: []info.Documentation{
					{
						Title: "Digital Video Ad Serving Template (VAST)",
						Link:  "https://www.iab.com/guidelines/vast/",
					},
				},
			},
			{
				Name:     "OpenNative",
				Protocol: "opennative",
				Versions: []string{"1.1", "1.2"},
				Docs: []info.Documentation{
					{
						Title: "OpenRTB Native Ads Specification 1.1",
						Link:  "https://www.iab.com/wp-content/uploads/2016/03/OpenRTB-Native-Ads-Specification-1-1_2016.pdf",
					},
					{
						Title: "OpenRTB Native Ads Specification 1.2",
						Link:  "https://www.iab.com/wp-content/uploads/2018/03/OpenRTB-Native-Ads-Specification-Final-1.2.pdf",
					},
				},
			},
		},
	}
}

func (*factory) Protocols() []string {
	return []string{"openrtb", "openrtb2", "openrtb3"}
}
