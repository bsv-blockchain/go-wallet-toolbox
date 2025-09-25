package actions

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/must"
)

// ListFailedActions lists only actions with status 'failed'.
func (l *listActions) ListFailedActions(ctx context.Context, auth wdk.AuthID, args *wdk.ListFailedActionsArgs) (*wdk.ListActionsResult, error) {
	userID := *auth.UserID

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

	filter.Status = []wdk.TxStatus{wdk.TxStatusFailed}

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
	if err := l.mapInputsOutputsLabels(actions, txs, inputMap, outputMap, labelMap, rawTxMap, mappingArgs); err != nil {
		return nil, fmt.Errorf("failed to map inputs/outputs/labels: %w", err)
	}

	if args.Unfail.Value() {
		for _, a := range actions {
			if a.TxID == "" {
				continue
			}
			if err := l.knownTxRepo.UpdateKnownTxStatus(ctx, a.TxID, wdk.ProvenTxStatusUnfail, nil, nil); err != nil {
				return nil, fmt.Errorf("failed to update known tx status: %w", err)
			}
		}
	}

	return &wdk.ListActionsResult{
		TotalActions: primitives.PositiveInteger(must.ConvertToUInt64(total)),
		Actions:      actions,
	}, nil
}
