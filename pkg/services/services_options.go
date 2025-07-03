package services

import (
	"net/http"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/options"
	"github.com/go-resty/resty/v2"
)

// WithHttpClient sets the http client for the service.
func WithHttpClient(client *http.Client) func(*options.Service) {
	r := resty.NewWithClient(client)
	return WithRestyClient(r)
}

// WithRestyClient sets the resty client for the WalletServices.
func WithRestyClient(client *resty.Client) func(*options.Service) {
	if client == nil {
		panic("client cannot be nil")
	}
	return func(o *options.Service) {
		o.RestyClientFactory = httpx.NewRestyClientFactoryWithBase(client)
	}
}
