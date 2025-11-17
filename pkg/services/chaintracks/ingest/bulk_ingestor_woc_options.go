package ingest

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/go-resty/resty/v2"
)

type BulkIngestorWocOptions struct {
	RestyClientFactory *httpx.RestyClientFactory
	APIKey             string
}

func DefaultBulkIngestorWocOptions() BulkIngestorWocOptions {
	return BulkIngestorWocOptions{
		RestyClientFactory: httpx.NewRestyClientFactory(),
	}
}

type BulkIngestorWocOptionsBuilder struct{}

var BulkIngestorWocOpts BulkIngestorWocOptionsBuilder

func (BulkIngestorWocOptionsBuilder) WithRestyClient(client *resty.Client) func(*BulkIngestorWocOptions) {
	return func(options *BulkIngestorWocOptions) {
		options.RestyClientFactory = httpx.NewRestyClientFactoryWithBase(client)
	}
}

func (BulkIngestorWocOptionsBuilder) WithAPIKey(apiKey string) func(*BulkIngestorWocOptions) {
	return func(options *BulkIngestorWocOptions) {
		options.APIKey = apiKey
	}
}
