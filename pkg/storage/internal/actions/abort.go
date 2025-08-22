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
	outputsRepo      OutputRepo
	utxosRepo        UTXORepo
	knownTxRepo      KnownTxRepo
}

const (
	txIDLength = 64
)

func newAbortAction(logger *slog.Logger, transactions TransactionsRepo, outputsRepo OutputRepo, utxosRepo UTXORepo, knownTxRepo KnownTxRepo) *abortAction {
	return &abortAction{
		logger:           logging.Child(logger, "abortAction"),
		transactionsRepo: transactions,
		outputsRepo:      outputsRepo,
		utxosRepo:        utxosRepo,
		knownTxRepo:      knownTxRepo,
	}
}

func (a *abortAction) AbortAction(ctx context.Context, userID int, args *wdk.AbortActionArgs) (*wdk.AbortActionResult, error) {
	referenceStr := string(args.Reference)

	txEntity, err := a.transactionsRepo.FindTransactionByReference(ctx, userID, referenceStr)
	if err != nil {
		return nil, fmt.Errorf("failed to find transaction by reference %s: %w", referenceStr, err)
	}

	if txEntity == nil && a.isPotentiallyTxID(referenceStr) {
		txEntity, err = a.transactionsRepo.FindTransactionByUserIDAndTxID(ctx, userID, referenceStr)
		if err != nil {
			return nil, fmt.Errorf("failed to find transaction by txid %s: %w", referenceStr, err)
		}
	}

	if txEntity == nil {
		return nil, fmt.Errorf("no transaction found with reference or txid %q", referenceStr)
	}

	if err := a.validateTx(txEntity); err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}

	if err := a.outputsRepo.IsAnyOutputOfTransactionSpent(ctx, txEntity.ID); err != nil {
		return nil, fmt.Errorf("cannot abort transaction with spent outputs: %w", err)
	}

	if err := a.utxosRepo.UnreserveUTXOsByTransactionID(ctx, txEntity.ID); err != nil {
		return nil, fmt.Errorf("failed to unreserve UTXOs for transaction: %w", err)
	}

	if err := a.outputsRepo.RecreateSpentOutputs(ctx, txEntity.ID); err != nil {
		return nil, fmt.Errorf("failed to recreate spent outputs for transaction: %w", err)
	}

	if err := a.transactionsRepo.UpdateTransactionStatusByID(ctx, txEntity.ID, wdk.TxStatusFailed); err != nil {
		return nil, fmt.Errorf("failed to update transaction status: %w", err)
	}

	// TODO: KnownTx is not tauched here because the same transaction can be owend by another user and we don't want to affect their state.
	// NOTE: The abandoned knownTx will be updated to failed by cron job

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
