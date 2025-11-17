package ingest

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/go-resty/resty/v2"
)

// IngestorWocPollOptions defines configuration settings for polling external data in the WOC ingestor component.
type IngestorWocPollOptions struct {
	RestyClientFactory *httpx.RestyClientFactory
	SyncPeriod         time.Duration
	APIKey             string
}

// DefaultIngestorWocPollOptions returns default options for polling block headers from an external source.
// Sets a Resty client factory and a 60-second sync period in the returned IngestorWocPollOptions struct.
func DefaultIngestorWocPollOptions() IngestorWocPollOptions {
	return IngestorWocPollOptions{
		RestyClientFactory: httpx.NewRestyClientFactory(),
		SyncPeriod:         60 * time.Second,
	}
}

// IngestorWocPollOptionsBuilder provides methods to configure options for polling data ingestion.
type IngestorWocPollOptionsBuilder struct{}

// IngestorWocPollOpts provides builder methods for configuring polling options with WOC-based ingestors.
var IngestorWocPollOpts IngestorWocPollOptionsBuilder

// WithRestyClient sets a custom resty.Client instance for the underlying HTTP client in polling options.
// It returns a functional option to configure IngestorWocPollOptions with the provided Resty client for network requests.
func (IngestorWocPollOptionsBuilder) WithRestyClient(client *resty.Client) func(*IngestorWocPollOptions) {
	return func(options *IngestorWocPollOptions) {
		options.RestyClientFactory = httpx.NewRestyClientFactoryWithBase(client)
	}
}

// WithSyncPeriod sets the synchronization interval for polling in the ingestor options builder.
func (IngestorWocPollOptionsBuilder) WithSyncPeriod(period time.Duration) func(*IngestorWocPollOptions) {
	return func(options *IngestorWocPollOptions) {
		options.SyncPeriod = period
	}
}

// WithAPIKey sets the API key for authenticating requests to the data provider in the ingest polling options builder.
func (IngestorWocPollOptionsBuilder) WithAPIKey(apiKey string) func(*IngestorWocPollOptions) {
	return func(options *IngestorWocPollOptions) {
		options.APIKey = apiKey
	}
}
