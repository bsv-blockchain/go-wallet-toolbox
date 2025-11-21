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

func createBulkIngestors(logger *slog.Logger, config defs.ChaintracksServiceConfig, initializers Initializers) []NamedBulkIngestor {
	logger.Info("Chaintracks service - creating bulk ingestors", slog.Any("configured_sources", config.BulkIngestors))

	ingestors := make([]NamedBulkIngestor, 0, len(config.BulkIngestors))
	for _, ingestorConfig := range config.BulkIngestors {
		switch ingestorConfig.Type {
		case defs.ChaintracksCDN:
			ingestor := initializers.CDNBulkIngestorFactory(logger, config.Chain, *ingestorConfig.CDNConfig)
			ingestors = append(ingestors, NamedBulkIngestor{
				Name:     ingestorConfig.String(),
				Ingestor: ingestor,
			})
		case defs.WhatsOnChainCDN:
			ingestor := initializers.WOCBulkIngestorFactory(logger, config.Chain, config.WocAPIKey)
			ingestors = append(ingestors, NamedBulkIngestor{
				Name:     ingestorConfig.String(),
				Ingestor: ingestor,
			})
		default:
			logger.Warn("Chaintracks service - unsupported bulk ingestor type, skipping", slog.String("ingestor_type", string(ingestorConfig.Type)))
		}
	}

	return ingestors
}
