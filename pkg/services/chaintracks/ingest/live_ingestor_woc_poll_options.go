package ingest

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/go-resty/resty/v2"
)

// ClientOptions holds optional configuration for customizing client behavior, such as injecting a custom RestyClientFactory.
type ClientOptions struct {
	RestyClientFactory *httpx.RestyClientFactory
	SyncPeriod         time.Duration
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

// WithSyncPeriod sets a custom sync period for use with ClientOptions.
func WithSyncPeriod(period time.Duration) func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.SyncPeriod = period
	}
}

// DefaultClientOptions returns a ClientOptions struct initialized with default RestyClientFactory and SyncPeriod values.
func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		RestyClientFactory: httpx.NewRestyClientFactory(),
		SyncPeriod:         60 * time.Second,
	}
}
