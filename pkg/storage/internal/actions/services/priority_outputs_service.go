package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type PriorityOutputsServiceRepository interface {
	FindOutputsByOutpoints(ctx context.Context, userID int, outpoints []wdk.OutPoint) ([]*entity.Output, error)
}

type PriorityOutputsService struct {
	logger     *slog.Logger
	repository PriorityOutputsServiceRepository
}

func (p *PriorityOutputsService) CreateOutputs(ctx context.Context, userID int, isNoSend bool, noSendChange []wdk.OutPoint) ([]*entity.Output, error) {
	logger := p.logger.With(
		slog.String("service", "priority_outputs_service"),
		slog.String("service_method", "create_outputs"),
		slog.Bool("is_no_send_param", isNoSend),
		slog.Int("no_send_change_len", len(noSendChange)),
		logging.UserID(userID),
	)

	if isNoSend && len(noSendChange) == 0 {
		logger.DebugContext(ctx, "Processing terminated immediately due to arguments values")
		return nil, nil
	}

	outputs, err := p.repository.FindOutputsByOutpoints(ctx, userID, noSendChange)
	if err != nil {
		return nil, fmt.Errorf("failed to find outputs by outpoints: %w", err)
	}

	logger = logger.With(
		slog.String("component", "repository"),
		slog.String("component_method", "find_outputs_by_outpoints"))

	logger.DebugContext(ctx, "Entity outputs successfully returned from the repository")

	if len(noSendChange) != len(outputs) {
		return nil, fmt.Errorf("failed to validate outputs: the number of outputs (%d) doesn't match the number of outpoints (%d)", len(outputs), len(noSendChange))
	}

	err = validate.NoSendChangeOutputs(outputs)
	if err != nil {
		return nil, fmt.Errorf("failed to validate no send change outputs: %w", err)
	}

	logger = logger.With(
		slog.String("component", "no_send_change_outputs_validator"),
		slog.String("component_method", "no_send_change_outputs"))

	logger.DebugContext(ctx, "Entity outputs (no send change outputs) successfully validated")

	return outputs, nil
}

func NewPriorityOutputsService(repository PriorityOutputsServiceRepository, logger *slog.Logger) *PriorityOutputsService {
	if repository == nil {
		panic("priority outputs service repository must be a non nil impl")
	}
	if logger == nil {
		panic("priority outputs service logger must be a non nil impl")
	}

	return &PriorityOutputsService{repository: repository, logger: logger}
}
