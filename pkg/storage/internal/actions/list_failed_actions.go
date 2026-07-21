package actions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/go-softwarelab/common/pkg/must"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// ListFailedActions lists actions with a terminal-unsuccessful status ('failed' or 'aborted').
func (l *listActions) ListFailedActions(ctx context.Context, auth wdk.AuthID, args *wdk.ListFailedActionsArgs) (*wdk.ListActionsResult, error) {
	userID := *auth.UserID
	var err error
	ctx, span := tracing.StartTracing(ctx, "StorageActions-ListFailedActions", attribute.Int("userID", userID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	filter, err := l.toFilterParams(userID, &wdk.ListActionsArgs{
		Labels:                           nil,
		Limit:                            args.Limit,
		Offset:                           args.Offset,
		LabelQueryMode:                   args.LabelQueryMode,
		SeekPermission:                   args.SeekPermission,
		IncludeInputs:                    args.IncludeInputs,
		IncludeOutputs:                   args.IncludeOutputs,
		IncludeLabels:                    args.IncludeLabels,
		IncludeInputSourceLockingScripts: args.IncludeInputSourceLockingScripts,
		IncludeInputUnlockingScripts:     args.IncludeInputUnlockingScripts,
		IncludeOutputLockingScripts:      args.IncludeOutputLockingScripts,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to convert filter params: %w", err)
	}

	filter.Status = []wdk.TxStatus{wdk.TxStatusFailed, wdk.TxStatusAborted}

	txs, total, err := l.transactionsRepo.ListAndCountActions(ctx, userID, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}

	transactionIDs, txIDs, actions := l.mapTransactionsToActions(txs)

	fetchArgs := &wdk.ListActionsArgs{
		IncludeInputs:               args.IncludeInputs,
		IncludeOutputs:              args.IncludeOutputs,
		IncludeOutputLockingScripts: args.IncludeOutputLockingScripts,
	}
	inputMap, outputMap, err := l.fetchInputsOutputs(ctx, transactionIDs, fetchArgs)
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

	mappingArgs := &wdk.ListActionsArgs{
		IncludeInputs:                    args.IncludeInputs,
		IncludeOutputs:                   args.IncludeOutputs,
		IncludeLabels:                    args.IncludeLabels,
		IncludeInputSourceLockingScripts: args.IncludeInputSourceLockingScripts,
		IncludeInputUnlockingScripts:     args.IncludeInputUnlockingScripts,
	}
	if err := l.mapInputsOutputsLabels(actions, txs, mapActionDetails{
		inputMap:  inputMap,
		outputMap: outputMap,
		labelMap:  labelMap,
		rawTxMap:  rawTxMap,
	}, mappingArgs, filter.TimeFilterRequested); err != nil {
		return nil, fmt.Errorf("failed to map inputs/outputs/labels: %w", err)
	}

	if args.Unfail.Value() {
		if err := l.markActionsForUnfail(ctx, actions); err != nil {
			return nil, err
		}
	}

	return &wdk.ListActionsResult{
		TotalActions: primitives.PositiveInteger(must.ConvertToUInt64(total)),
		Actions:      actions,
	}, nil
}

// markActionsForUnfail flips each listed action's KnownTx to 'unfail' so the UnFail
// cron re-verifies it. Only genuinely 'failed' actions are eligible: aborted (and any
// other non-'failed') actions are skipped outright because they were never broadcast,
// so there is nothing on-chain to re-verify - flipping their (possibly shared) KnownTx
// to 'unfail' would be wrong and, worse, would let the UnFail cron later cascade it
// back to 'failed', silently re-erasing the aborted status. Skipped updates for the
// remaining, eligible actions are also legitimate: a failed Transaction can have a
// tx_id with no matching KnownTx row (e.g. the abort sweep's own filter tolerates
// such rows via COALESCE(known_txs.status,'unprocessed')) — there is nothing to
// unfail for that tx, so it is logged and the remaining actions are processed.
func (l *listActions) markActionsForUnfail(ctx context.Context, actions []wdk.WalletAction) error {
	for _, a := range actions {
		if a.Status != string(wdk.TxStatusFailed) {
			continue
		}
		if a.TxID == "" {
			continue
		}
		err := l.knownTxRepo.UpdateKnownTxStatus(ctx, a.TxID, wdk.ProvenTxStatusUnfail, nil, nil)
		if err == nil {
			continue
		}
		if errors.Is(err, repo.ErrStatusUpdateSkipped) {
			l.logger.DebugContext(ctx, "known tx status update skipped for unfail (no matching KnownTx row)", slog.String("txID", a.TxID), logging.Error(err))
			continue
		}
		return fmt.Errorf("failed to update known tx status: %w", err)
	}
	return nil
}
