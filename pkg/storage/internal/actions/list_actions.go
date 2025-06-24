package actions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
)

type listActions struct {
	logger           *slog.Logger
	transactionsRepo TransactionsRepo
	outputsRepo      OutputRepo
	knownTxRepo      KnownTxRepo
	basketRepo       BasketRepo
}

func newListActions(logger *slog.Logger, transactions TransactionsRepo, outputs OutputRepo, knownTxRepo KnownTxRepo, basket BasketRepo) *listActions {
	return &listActions{
		logger:           logging.Child(logger, "list_actions"),
		transactionsRepo: transactions,
		outputsRepo:      outputs,
		knownTxRepo:      knownTxRepo,
		basketRepo:       basket,
	}
}

func (l *listActions) ListActions(ctx context.Context, auth wdk.AuthID, args *wdk.ListActionsArgs) (*wdk.ListActionsResult, error) {
	userID := *auth.UserID
	filter, err := l.toFilterParams(userID, args)
	if err != nil {
		return nil, fmt.Errorf("failed to convert filter params: %w", err)
	}

	txs, total, err := l.transactionsRepo.ListAndCountActions(ctx, userID, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}

	transactionIDs, txIDs, actions := l.mapTransactionsToActions(txs)

	inputMap, outputMap, err := l.fetchInputsOutputs(ctx, transactionIDs, args)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch inputs/outputs: %w", err)
	}

	labelMap, err := l.loadLabelsIfNeeded(ctx, transactionIDs, args.IncludeLabels)
	if err != nil {
		return nil, fmt.Errorf("failed to load labels: %w", err)
	}

	rawTxMap, err := l.loadRawTxsIfNeeded(ctx, txIDs, args.IncludeInputs)
	if err != nil {
		return nil, fmt.Errorf("failed to load raw transactions: %w", err)
	}

	if err := l.mapInputsOutputsLabels(actions, txs, inputMap, outputMap, labelMap, rawTxMap, args); err != nil {
		return nil, fmt.Errorf("failed to map inputs/outputs/labels: %w", err)
	}

	return &wdk.ListActionsResult{
		TotalActions: must.ConvertToUInt64(total),
		Actions:      actions,
	}, nil
}
