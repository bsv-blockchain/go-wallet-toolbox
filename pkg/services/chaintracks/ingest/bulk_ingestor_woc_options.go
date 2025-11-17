package ingest

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/go-resty/resty/v2"
)

// BulkIngestorWocOptions provides configuration for bulk ingestion using a configurable HTTP client and optional API key.
type BulkIngestorWocOptions struct {
	RestyClientFactory *httpx.RestyClientFactory
	APIKey             string
}

// DefaultBulkIngestorWocOptions returns the default BulkIngestorWocOptions with a configured RestyClientFactory.
func DefaultBulkIngestorWocOptions() BulkIngestorWocOptions {
	return BulkIngestorWocOptions{
		RestyClientFactory: httpx.NewRestyClientFactory(),
	}
}

// BulkIngestorWocOptionsBuilder provides builder methods to configure BulkIngestorWocOptions for bulk ingestion.
type BulkIngestorWocOptionsBuilder struct{}

// BulkIngestorWocOpts provides option builder methods for customizing BulkIngestorWocOptions configuration.
var BulkIngestorWocOpts BulkIngestorWocOptionsBuilder

// WithRestyClient sets a custom resty.Client to be used for HTTP requests in BulkIngestorWocOptions.
// It overrides the default RestyClientFactory with one based on the provided client instance.
func (BulkIngestorWocOptionsBuilder) WithRestyClient(client *resty.Client) func(*BulkIngestorWocOptions) {
	return func(options *BulkIngestorWocOptions) {
		options.RestyClientFactory = httpx.NewRestyClientFactoryWithBase(client)
	}
}

// WithAPIKey sets the API key to be used with the BulkIngestorWocOptions instance.
func (BulkIngestorWocOptionsBuilder) WithAPIKey(apiKey string) func(*BulkIngestorWocOptions) {
	return func(options *BulkIngestorWocOptions) {
		options.APIKey = apiKey
	}
}
