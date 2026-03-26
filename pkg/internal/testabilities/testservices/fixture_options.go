package testservices

import (
	"github.com/jarcoal/httpmock"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

type FixtureOptions struct {
	transport *httpmock.MockTransport
	network   defs.BSVNetwork
}

type Option = func(*FixtureOptions)

func WithTransport(transport *httpmock.MockTransport) Option {
	if transport == nil {
		panic("transport cannot be nil")
	}
	return func(o *FixtureOptions) {
		o.transport = transport
	}
}

func WithNetwork(network defs.BSVNetwork) Option {
	return func(o *FixtureOptions) {
		o.network = network
	}
}
