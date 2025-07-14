package actions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
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

	if txEntity == nil {
		return nil, fmt.Errorf("failed to find unique transaction by reference: expected exactly one transaction with reference %s, found 0", *args.Reference)
	}

	err = validateTxForAbort(txEntity)
	if err != nil {
		return nil, err
	}

	historyNote := "abortAction"
	historyAttrs := map[string]any{
		"action":    "abort_action",
		"reference": *args.Reference,
	}

	if txEntity.TxID != nil {
		err = a.transactionsRepo.UpdateTransactionStatusForTxID(ctx, *txEntity.TxID, wdk.TxStatusFailed, wdk.ProvenTxStatusInvalid, historyNote, historyAttrs)
		if err != nil {
			a.logger.Warn("Failed to update known transaction status",
				slog.String("txID", *txEntity.TxID),
				slog.String("error", err.Error()))
			return nil, fmt.Errorf("failed to update known transaction status: %w", err)
		}
	}

	err = a.transactionsRepo.UpdateTransactionStatusForID(ctx, txEntity.ID, wdk.TxStatusFailed, wdk.ProvenTxStatusInvalid, historyNote, historyAttrs)
	if err != nil {
		return nil, fmt.Errorf("failed to update transaction status: %w", err)
	}

	return &wdk.AbortActionResult{
		Aborted: true,
	}, nil
}

func validateTxForAbort(txEntity *entity.Transaction) error {
	if !txEntity.IsOutgoing {
		return fmt.Errorf("txStatusInvalid: reference must be an outgoing action.")
	}

	return validateTxStatusForAbort(txEntity.Status)
}

func validateTxStatusForAbort(txStatus wdk.TxStatus) error {
	var unAbortableStatuses = map[wdk.TxStatus]bool{
		wdk.TxStatusCompleted: true,
		wdk.TxStatusFailed:    true,
		wdk.TxStatusSending:   true,
		wdk.TxStatusUnproven:  true,
	}

	if unAbortableStatuses[txStatus] {
		return fmt.Errorf("txStatusInvalid: action with status %s cannot be aborted", txStatus)
	}

	return nil
}
