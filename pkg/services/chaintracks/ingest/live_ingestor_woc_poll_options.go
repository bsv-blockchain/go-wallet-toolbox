package ingest

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/go-resty/resty/v2"
)

type ClientOptions struct {
	RestyClientFactory *httpx.RestyClientFactory
}

func WithRestyClient(client *resty.Client) func(*ClientOptions) {
	if client == nil {
		panic("client cannot be nil")
	}
	return func(o *ClientOptions) {
		o.RestyClientFactory = httpx.NewRestyClientFactoryWithBase(client)
	}
}
