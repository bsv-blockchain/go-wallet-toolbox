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

const (
	txIDLength = 64
)

func newAbortAction(logger *slog.Logger, transactions TransactionsRepo) *abortAction {
	return &abortAction{
		logger:           logging.Child(logger, "abortAction"),
		transactionsRepo: transactions,
	}
}

func (a *abortAction) AbortAction(ctx context.Context, userID int, args *wdk.AbortActionArgs) (*wdk.AbortActionResult, error) {
	txEntity, err := a.transactionsRepo.FindTransactionByReference(ctx, userID, args.Reference)
	if err != nil {
		return nil, fmt.Errorf("failed to find unique transaction by reference: %w", err)
	}

	if txEntity == nil && a.isPotentiallyTxID(args.Reference) {
		txEntity, err = a.transactionsRepo.FindTransactionByUserIDAndTxID(ctx, userID, args.Reference)
		if err != nil {
			return nil, fmt.Errorf("failed to find transaction by txid: %w", err)
		}
	}
	if txEntity == nil {
		return nil, fmt.Errorf("no transaction found with reference or txid %q", args.Reference)
	}
	if err := a.validateTx(txEntity); err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}
	if err := a.transactionsRepo.AbortTransactionAtomic(ctx, txEntity.ID, txEntity.TxID, args.Reference); err != nil {
		return nil, fmt.Errorf("failed to abort transaction: %w", err)
	}
	return &wdk.AbortActionResult{Aborted: true}, nil
}

func (a *abortAction) validateTx(txEntity *entity.Transaction) error {
	if !txEntity.IsOutgoing {
		return fmt.Errorf("%w: must be an outgoing transaction", wdk.ErrNotAbortableAction)
	}

	return validateTxStatusForAbort(txEntity.Status)
}

func validateTxStatusForAbort(txStatus wdk.TxStatus) error {
	switch txStatus {
	case wdk.TxStatusCompleted, wdk.TxStatusFailed, wdk.TxStatusSending, wdk.TxStatusUnproven:
		return fmt.Errorf("%w: action with status %s cannot be aborted", wdk.ErrNotAbortableAction, txStatus)
	case wdk.TxStatusUnprocessed, wdk.TxStatusUnsigned, wdk.TxStatusNoSend, wdk.TxStatusNonFinal, wdk.TxStatusUnfail:
		return nil
	default:
		return fmt.Errorf("%w: unexpected transaction status %s", wdk.ErrNotAbortableAction, txStatus)
	}
}

func (a *abortAction) isPotentiallyTxID(reference string) bool {
	return len(reference) == txIDLength
}
