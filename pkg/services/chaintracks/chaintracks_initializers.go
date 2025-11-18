package chaintracks

import (
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest"
)

// Initializers provides injectable factory functions for customizing live ingestor creation in the Chaintracks service.
// Use this struct to override default implementations, especially for testing or custom runtime behaviors.
type Initializers struct {
	WOCLiveIngestorPollFactory func(logger *slog.Logger, config defs.ChaintracksServiceConfig) LiveIngestor
	CDNBulkIngestorFactory     func(logger *slog.Logger, chain defs.BSVNetwork, config defs.CDNBulkIngestorConfig) BulkIngestor
	WOCBulkIngestorFactory     func(logger *slog.Logger, chain defs.BSVNetwork, apiKey string) BulkIngestor
}

// DefaultInitializers returns an Initializers struct with the default WOCLiveIngestorPollFactory implementation.
func DefaultInitializers() Initializers {
	return Initializers{
		WOCLiveIngestorPollFactory: func(logger *slog.Logger, config defs.ChaintracksServiceConfig) LiveIngestor {
			return ingest.NewLiveIngestorWocPoll(logger, config.Chain, ingest.IngestorWocPollOpts.WithAPIKey(config.WocAPIKey))
		},
		CDNBulkIngestorFactory: func(logger *slog.Logger, chain defs.BSVNetwork, config defs.CDNBulkIngestorConfig) BulkIngestor {
			return ingest.NewBulkIngestorCDN(logger, chain, config)
		},
		WOCBulkIngestorFactory: func(logger *slog.Logger, chain defs.BSVNetwork, apiKey string) BulkIngestor {
			return ingest.NewBulkIngestorWOC(logger, chain, ingest.BulkIngestorWocOpts.WithAPIKey(apiKey))
		},
	}
}

func createInitializers(inits ...Initializers) Initializers {
	finalInits := DefaultInitializers()

	for _, in := range inits {
		if in.WOCLiveIngestorPollFactory != nil {
			finalInits.WOCLiveIngestorPollFactory = in.WOCLiveIngestorPollFactory
		}
		if in.CDNBulkIngestorFactory != nil {
			finalInits.CDNBulkIngestorFactory = in.CDNBulkIngestorFactory
		}
		if in.WOCBulkIngestorFactory != nil {
			finalInits.WOCBulkIngestorFactory = in.WOCBulkIngestorFactory
		}
	}

	return finalInits
}
