package actions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type abortAction struct {
	logger           *slog.Logger
	transactionsRepo TransactionsRepo
}

func newAbortAction(logger *slog.Logger, transactions TransactionsRepo) *abortAction {
	return &abortAction{
		logger:           logging.Child(logger, "abort_action"),
		transactionsRepo: transactions,
	}
}

func (a *abortAction) AbortAction(ctx context.Context, userID int, args *wdk.AbortActionArgs) (*wdk.AbortActionResult, error) {
	txEntity, err := a.transactionsRepo.FindUniqueTransactionByReference(ctx, userID, *args.Reference)
	if err != nil {
		return nil, fmt.Errorf("failed to find unique transaction by reference: %w", err)
	}

	if txEntity == nil && len(*args.Reference) == 64 {
		txEntity, err = a.transactionsRepo.FindTransactionByUserIDAndTxID(ctx, userID, *args.Reference)
		if err != nil {
			return nil, fmt.Errorf("failed to find transaction by txid: %w", err)
		}
	}

	err = validateTxStatusForAbort(txEntity.Status)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction status for abort: %w", err)
	}

	historyNote := "abortAction"
	historyAttrs := map[string]any{
		"action":    "abort_action",
		"reference": *args.Reference,
	}
	a.transactionsRepo.UpdateTransactionStatusForTxID(
		ctx,
		*txEntity.TxID,
		wdk.TxStatusFailed,
		wdk.ProvenTxStatusInvalid,
		historyNote,
		historyAttrs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update transaction status: %w", err)
	}

	return &wdk.AbortActionResult{
		Aborted: true,
	}, nil
}

func validateTxStatusForAbort(txStatus wdk.TxStatus) error {
	var validAbortStatuses = map[wdk.TxStatus]bool{
		wdk.TxStatusCompleted: true,
		wdk.TxStatusFailed:    true,
		wdk.TxStatusUnproven:  true,
		wdk.TxStatusSending:   true,
	}

	if validAbortStatuses[txStatus] {
		return fmt.Errorf("transaction status %s is not valid for aborting", txStatus)
	}

	return nil
}
