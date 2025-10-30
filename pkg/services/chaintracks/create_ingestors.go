package chaintracks

import (
	"log/slog"
	"maps"
	"slices"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest"
)

func createLiveIngestors(logger *slog.Logger, config defs.ChaintracksServiceConfig) []NamedLiveIngestor {
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
				Ingestor: ingest.NewLiveIngestorWocPoll(logger, defs.WOCPollIngestorConfig{ Chain: config.Chain }),
			}
		default:
			logger.Warn("Chaintracks service - unsupported live ingestor type, skipping", slog.String("ingestor_type", string(ingestorType)))
		}
	}

	return slices.AppendSeq(make([]NamedLiveIngestor, 0, len(ingestorsMap)), maps.Values(ingestorsMap))
}
