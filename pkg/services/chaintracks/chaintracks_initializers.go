package chaintracks

import (
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest"
)

// Initializers provides injectable factory functions for customizing live ingestor creation in the Chaintracks service.
// Use this struct to override default implementations, especially for testing or custom runtime behaviors.
type Initializers struct{
	WOCLiveIngestorPollFactory func(logger *slog.Logger, config defs.ChaintracksServiceConfig) LiveIngestor
}

// DefaultInitializers returns an Initializers struct with the default WOCLiveIngestorPollFactory implementation.
func DefaultInitializers() Initializers {
	return Initializers{
		WOCLiveIngestorPollFactory: func(logger *slog.Logger, config defs.ChaintracksServiceConfig) LiveIngestor {
			return ingest.NewLiveIngestorWocPoll(logger, defs.WOCPollIngestorConfig{Chain: config.Chain})
		},
	}
}

func createInitializers(inits ...Initializers) Initializers {
	finalInits := DefaultInitializers()

	for _, in := range inits {
		if in.WOCLiveIngestorPollFactory != nil {
			finalInits.WOCLiveIngestorPollFactory = in.WOCLiveIngestorPollFactory
		}
	}

	return finalInits
}
