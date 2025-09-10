package services

import (
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/go-resty/resty/v2"
)

type Options struct {
	RestyClientFactory      *httpx.RestyClientFactory
	RawTxMethodsModifier    func([]NamedFunc[RawTxFunc]) []NamedFunc[RawTxFunc]
	PostBEEFMethodsModifier func([]NamedFunc[PostBEEFFunc]) []NamedFunc[PostBEEFFunc]

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

func WithServicesDefinition(priority int, name string, servicesDef AllServicesDefinition) func(*Options) {
	return func(o *Options) {
		o.servicesDefinitionItems = append(o.servicesDefinitionItems, allServicesDefinitionItem{
			AllServicesDefinition: servicesDef,
			Priority:              priority,
			Name:                  name,
		})
	}
}

func WithRawTxMethodsModifier(modifier func([]NamedFunc[RawTxFunc]) []NamedFunc[RawTxFunc]) func(*Options) {
	return func(o *Options) {
		o.RawTxMethodsModifier = modifier
	}
}

func WithPostBEEFMethodsModifier(modifier func([]NamedFunc[PostBEEFFunc]) []NamedFunc[PostBEEFFunc]) func(*Options) {
	return func(o *Options) {
		o.PostBEEFMethodsModifier = modifier
	}
}
