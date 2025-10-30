package chaintracks

import (
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest"
)

type Initializers struct{
	WOCLiveIngestorPollFactory func(logger *slog.Logger, config defs.ChaintracksServiceConfig) LiveIngestor
}

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
