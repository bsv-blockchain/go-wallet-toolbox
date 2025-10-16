package chaintracks

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/go-resty/resty/v2"
)

// ClientOptions defines configuration settings for creating a ChainTracks client, including HTTP REST client factory.
type ClientOptions struct {
	RestyClientFactory *httpx.RestyClientFactory
}

// WithRestyClient sets a custom resty.Client to be used by the ChainTracks client via ClientOptions.
// Panics if the given resty.Client is nil.
// Returns a function that configures ClientOptions with the provided resty.Client instance.
func WithRestyClient(client *resty.Client) func(*ClientOptions) {
	if client == nil {
		panic("client cannot be nil")
	}
	return func(o *ClientOptions) {
		o.RestyClientFactory = httpx.NewRestyClientFactoryWithBase(client)
	}
}
