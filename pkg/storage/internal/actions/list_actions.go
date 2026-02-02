package actions

import (
	"context"
	"fmt"
	"log/slog"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
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
	var err error
	ctx, span := tracing.StartTracing(ctx, "StorageActions-Internalize")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	filter, err := l.toFilterParams(userID, args)
	if err != nil {
		return nil, fmt.Errorf("failed to convert filter params: %w", err)
	}

	txs, total, err := l.transactionsRepo.ListAndCountActions(ctx, userID, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}

	_, txIDs, actions := l.mapTransactionsToActions(txs)

	var inputMap map[uint][]*pkgentity.Output
	var outputMap map[uint][]*pkgentity.Output
	if args.IncludeInputs.Value() || args.IncludeOutputs.Value() {
		inputMap, outputMap, err = l.outputsRepo.FindInputsAndOutputsForSelectedActions(ctx, userID, filter, args.IncludeOutputLockingScripts.Value())
		if err != nil {
			return nil, fmt.Errorf("failed to fetch inputs/outputs: %w", err)
		}
	} else {
		inputMap = map[uint][]*pkgentity.Output{}
		outputMap = map[uint][]*pkgentity.Output{}
	}

	var labelMap map[uint][]string
	if args.IncludeLabels.Value() {
		labelMap, err = l.transactionsRepo.GetLabelsForSelectedActions(ctx, userID, filter)
		if err != nil {
			return nil, fmt.Errorf("failed to load labels: %w", err)
		}
	} else {
		labelMap = map[uint][]string{}
	}
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
		TotalActions: primitives.PositiveInteger(must.ConvertToUInt64(total)),
		Actions:      actions,
	}, nil
}
