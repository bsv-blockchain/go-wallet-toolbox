package services

import (
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/go-resty/resty/v2"
)

type Options struct {
	RestyClientFactory      *httpx.RestyClientFactory
	RawTxMethodsModifier    func([]Named[RawTxFunc]) []Named[RawTxFunc]
	PostBEEFMethodsModifier func([]Named[PostBEEFFunc]) []Named[PostBEEFFunc]

	servicesDefinitionItems []allServicesDefinitionItem
}

// WithHttpClient sets the http client for the service.
func WithHttpClient(client *http.Client) func(*Options) {
	r := resty.NewWithClient(client)
	return WithRestyClient(r)
}

// WithRestyClient sets the resty client for the WalletServices.
func WithRestyClient(client *resty.Client) func(*Options) {
	if client == nil {
		panic("client cannot be nil")
	}
	return func(o *Options) {
		o.RestyClientFactory = httpx.NewRestyClientFactoryWithBase(client)
	}
}

func WithServicesDefinition(name string, servicesDef AllServicesDefinition) func(*Options) {
	return func(o *Options) {
		o.servicesDefinitionItems = append(o.servicesDefinitionItems, allServicesDefinitionItem{
			Name: name,
			Item: servicesDef,
		})
	}
}

func WithRawTxMethodsModifier(modifier func([]Named[RawTxFunc]) []Named[RawTxFunc]) func(*Options) {
	return func(o *Options) {
		o.RawTxMethodsModifier = modifier
	}
}

func WithPostBEEFMethodsModifier(modifier func([]Named[PostBEEFFunc]) []Named[PostBEEFFunc]) func(*Options) {
	return func(o *Options) {
		o.PostBEEFMethodsModifier = modifier
	}
}
