package chaintracks

import (
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func createLiveIngestors(logger *slog.Logger, config defs.ChaintracksServiceConfig, initializers Initializers) []NamedLiveIngestor {
	logger.Info("Chaintracks service - creating live ingestors", slog.Any("configured_types", config.LiveIngestors))

	ingestorsMap := make(map[defs.LiveIngestorType]NamedLiveIngestor)
	for _, ingestorType := range config.LiveIngestors {
		if _, exists := ingestorsMap[ingestorType]; exists {
			logger.Warn("Chaintracks service - duplicate live ingestor type configured, skipping", slog.String("ingestor_type", string(ingestorType)))
			continue
		}

		switch ingestorType {
		case defs.LiveIngestorTypeWocPoll:
			ingestorsMap[ingestorType] = NamedLiveIngestor{
				Name:     string(ingestorType),
				Ingestor: initializers.WOCLiveIngestorPollFactory(logger, config),
			}
		default:
			logger.Warn("Chaintracks service - unsupported live ingestor type, skipping", slog.String("ingestor_type", string(ingestorType)))
		}
	}

	// an order is needed:
	ingestors := make([]NamedLiveIngestor, 0, len(ingestorsMap))
	for _, ingestorType := range config.LiveIngestors {
		if ingestor, exists := ingestorsMap[ingestorType]; exists {
			ingestors = append(ingestors, ingestor)
			delete(ingestorsMap, ingestorType) // to avoid duplicates
		}
	}

	return ingestors
}
