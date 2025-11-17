package ingest

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/go-resty/resty/v2"
)

type IngestorWocPollOptions struct {
	RestyClientFactory *httpx.RestyClientFactory
	SyncPeriod         time.Duration
	APIKey             string
}

func DefaultIngestorWocPollOptions() IngestorWocPollOptions {
	return IngestorWocPollOptions{
		RestyClientFactory: httpx.NewRestyClientFactory(),
		SyncPeriod:         60 * time.Second,
	}
}

type IngestorWocPollOptionsBuilder struct{}

var IngestorWocPollOpts IngestorWocPollOptionsBuilder

func (IngestorWocPollOptionsBuilder) WithRestyClient(client *resty.Client) func(*IngestorWocPollOptions) {
	return func(options *IngestorWocPollOptions) {
		options.RestyClientFactory = httpx.NewRestyClientFactoryWithBase(client)
	}
}

func (IngestorWocPollOptionsBuilder) WithSyncPeriod(period time.Duration) func(*IngestorWocPollOptions) {
	return func(options *IngestorWocPollOptions) {
		options.SyncPeriod = period
	}
}
func (IngestorWocPollOptionsBuilder) WithAPIKey(apiKey string) func(*IngestorWocPollOptions) {
	return func(options *IngestorWocPollOptions) {
		options.APIKey = apiKey
	}
}
