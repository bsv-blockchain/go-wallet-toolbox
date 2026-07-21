package actions

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"go.opentelemetry.io/otel/attribute"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type abortAction struct {
	logger            *slog.Logger
	transactionsRepo  TransactionsRepo
	outputsRepo       OutputRepo
	utxosRepo         UTXORepo
	knownTxRepo       ProvenTxReqRepo
	failAbandonedLock sync.Mutex
	uow               UnitOfWork
}

const (
	txIDLength = 64
)

func newAbortAction(logger *slog.Logger, transactions TransactionsRepo, outputsRepo OutputRepo, utxosRepo UTXORepo, knownTxRepo ProvenTxReqRepo, uow UnitOfWork) *abortAction {
	return &abortAction{
		logger:           logging.Child(logger, "abortAction"),
		transactionsRepo: transactions,
		outputsRepo:      outputsRepo,
		utxosRepo:        utxosRepo,
		knownTxRepo:      knownTxRepo,
		uow:              uow,
	}
}

func (a *abortAction) AbortAction(ctx context.Context, userID int, args *wdk.AbortActionArgs) (*wdk.AbortActionResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "StorageActions-AbortAction", attribute.Int("userID", userID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	referenceStr := string(args.Reference)
	logger := a.logger.With(
		logging.UserID(userID),
		slog.String("reference", referenceStr),
	)

	logger.InfoContext(
		ctx, "Starting AbortAction process",
		slog.Bool("isPotentialTxID", a.isPotentiallyTxID(referenceStr)),
	)

	logger.DebugContext(ctx, "Searching for transaction by reference or txid")
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

	logger.DebugContext(ctx, "Checking if transaction was found")

	if txEntity == nil {
		return nil, fmt.Errorf("no transaction found with reference or txid %q", referenceStr)
	}

	logger.DebugContext(
		ctx, "Validating transaction for abort",
		logging.Number("transactionID", txEntity.ID),
		slog.String("status", string(txEntity.Status)),
		slog.Bool("isOutgoing", txEntity.IsOutgoing),
	)

	if err := a.validateTx(ctx, txEntity); err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}

	logger.DebugContext(
		ctx, "Starting transaction abort process",
		logging.Number("transactionID", txEntity.ID),
	)

	if err := a.abortTx(ctx, txEntity.ID); err != nil {
		return nil, fmt.Errorf("failed to abort transaction: %w", err)
	}

	logger.InfoContext(
		ctx, "AbortAction completed successfully",
		logging.Number("transactionID", txEntity.ID),
	)

	return &wdk.AbortActionResult{Aborted: true}, nil
}

func (a *abortAction) abortTx(ctx context.Context, id uint) error {
	logger := a.logger.With(logging.Number("transactionID", id))

	return a.uow.Do(ctx, func(txCtx context.Context, repos Providers) error {
		// NOTE: RecreateSpentOutputs below both restores spendability and clears SpentBy
		// for outputs consumed by this transaction, so no separate unreserve step is needed
		// here (running UnreserveUTXOsByTransactionID first would clear SpentBy and make
		// RecreateSpentOutputs' SpentBy-based lookup match nothing).
		logger.DebugContext(txCtx, "Recreating spent outputs for transaction")
		if err := repos.OutputRepo().RecreateSpentOutputs(txCtx, id); err != nil {
			return fmt.Errorf("failed to recreate spent outputs for transaction: %w", err)
		}

		logger.DebugContext(txCtx, "Marking created outputs as not spendable for transaction")
		if err := repos.OutputRepo().MarkCreatedOutputsAsNotSpendable(txCtx, id); err != nil {
			return fmt.Errorf("failed to mark created outputs as not spendable for transaction: %w", err)
		}

		logger.DebugContext(txCtx, "Updating transaction status to 'failed'")
		if err := repos.TransactionsRepo().UpdateTransactionStatusByID(txCtx, id, wdk.TxStatusFailed); err != nil {
			return fmt.Errorf("failed to update transaction status: %w", err)
		}

		// TODO: KnownTx is not touched here because the same transaction can be owend by another user and we don't want to affect their state.
		// NOTE: The abandoned knownTx will be updated to failed by cron job

		return nil
	})
}

func (a *abortAction) validateTx(ctx context.Context, txEntity *pkgentity.Transaction) error {
	logger := a.logger.With(
		logging.Number("transactionID", txEntity.ID),
		slog.Bool("isOutgoing", txEntity.IsOutgoing),
		slog.String("status", string(txEntity.Status)),
	)

	logger.DebugContext(ctx, "Validating if transaction is outgoing")
	if !txEntity.IsOutgoing {
		return fmt.Errorf("%w: must be an outgoing transaction", wdk.ErrNotAbortableAction)
	}

	logger.DebugContext(ctx, "Validating transaction status")
	if err := validateTxStatusForAbort(txEntity.Status); err != nil {
		return err
	}

	logger.DebugContext(ctx, "Checking if transaction outputs are unspent")
	if err := a.outputsRepo.ShouldTxOutputsBeUnspent(ctx, txEntity.ID); err != nil {
		return fmt.Errorf("cannot abort transaction with spent outputs: %w", err)
	}

	logger.DebugContext(ctx, "Transaction validation passed")
	return nil
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
