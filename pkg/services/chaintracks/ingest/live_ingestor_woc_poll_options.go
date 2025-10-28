package ingest

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/go-resty/resty/v2"
)

// ClientOptions holds optional configuration for customizing client behavior, such as injecting a custom RestyClientFactory.
type ClientOptions struct {
	RestyClientFactory *httpx.RestyClientFactory
}

// WithRestyClient sets a custom resty.Client for use with ClientOptions, panicking if the provided client is nil.
func WithRestyClient(client *resty.Client) func(*ClientOptions) {
	if client == nil {
		panic("client cannot be nil")
	}
	return func(o *ClientOptions) {
		o.RestyClientFactory = httpx.NewRestyClientFactoryWithBase(client)
	}
}
