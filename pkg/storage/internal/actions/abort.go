package actions

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"go.opentelemetry.io/otel/attribute"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type abortAction struct {
	logger            *slog.Logger
	transactionsRepo  TransactionsRepo
	outputsRepo       OutputRepo
	utxosRepo         UTXORepo
	knownTxRepo       KnownTxRepo
	failAbandonedLock sync.Mutex
	uow               UnitOfWork
}

const (
	txIDLength = 64
)

func newAbortAction(logger *slog.Logger, transactions TransactionsRepo, outputsRepo OutputRepo, utxosRepo UTXORepo, knownTxRepo KnownTxRepo, uow UnitOfWork) *abortAction {
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

	if err := a.abortTx(ctx, txEntity); err != nil {
		return nil, fmt.Errorf("failed to abort transaction: %w", err)
	}

	logger.InfoContext(
		ctx, "AbortAction completed successfully",
		logging.Number("transactionID", txEntity.ID),
	)

	return &wdk.AbortActionResult{Aborted: true}, nil
}

// abortTx releases one transaction: it parks the shared KnownTx (when the transaction has a
// txid) and then gives the reserved inputs back.
//
// Parking is the evidence gate of the whole abort: it only applies while the transaction
// provably never reached a broadcaster (never-posted status, was_broadcast false, no
// attempt), and once applied no pipeline can claim the transaction for a post anymore.
// A transaction that cannot be parked is therefore not abortable.
func (a *abortAction) abortTx(ctx context.Context, txEntity *pkgentity.Transaction) error {
	id := txEntity.ID
	logger := a.logger.With(logging.Number("transactionID", id))

	if txEntity.TxID != nil {
		if err := a.parkKnownTxForAbort(ctx, txEntity); err != nil {
			return err
		}
	}

	return a.uow.Do(ctx, func(txCtx context.Context, repos Providers) error {
		logger.DebugContext(txCtx, "Unreserving UTXOs for transaction")
		if err := repos.UTXORepo().UnreserveUTXOsByTransactionID(txCtx, id); err != nil {
			return fmt.Errorf("failed to unreserve UTXOs for transaction: %w", err)
		}

		logger.DebugContext(txCtx, "Recreating spent outputs for transaction")
		if err := repos.OutputRepo().RecreateSpentOutputs(txCtx, id); err != nil {
			return fmt.Errorf("failed to recreate spent outputs for transaction: %w", err)
		}

		logger.DebugContext(txCtx, "Marking created outputs as not spendable for transaction")
		if err := repos.OutputRepo().MarkCreatedOutputsAsNotSpendable(txCtx, id); err != nil {
			return fmt.Errorf("failed to mark created outputs as not spendable for transaction: %w", err)
		}

		logger.DebugContext(txCtx, "Updating transaction status to 'aborted'")
		// Positive CAS: only abort a transaction still in an abortable status. If it raced
		// to a non-abortable status between validation and here, the update matches zero rows
		// and returns ErrStatusUpdateSkipped, which propagates and rolls back the whole abort
		// UoW (the concurrent transition wins).
		if err := repos.TransactionsRepo().UpdateTransactionStatusByID(txCtx, id, wdk.TxStatusAborted,
			wdk.TxStatusUnprocessed, wdk.TxStatusUnsigned, wdk.TxStatusNoSend, wdk.TxStatusNonFinal, wdk.TxStatusUnfail); err != nil {
			return fmt.Errorf("failed to update transaction status: %w", err)
		}

		return nil
	})
}

// parkKnownTxForAbort parks the transaction's shared KnownTx so that no broadcast pipeline
// can claim it afterwards. It refuses when another owner still has the transaction in flight
// (their copy may be broadcast, so our inputs must stay spent) and when the KnownTx carries
// any evidence of a post.
func (a *abortAction) parkKnownTxForAbort(ctx context.Context, txEntity *pkgentity.Transaction) error {
	txID := *txEntity.TxID

	transactions, err := a.transactionsRepo.FindTransactions(ctx, &pkgentity.TransactionReadSpecification{
		TxID: &pkgentity.Comparable[string]{Value: txID, Cmp: pkgentity.Equal},
	})
	if err != nil {
		return fmt.Errorf("failed to find transactions for txID %s: %w", txID, err)
	}

	if _, othersStillActive := a.splitTxsForPreBroadcastAbort(transactions, &txEntity.UserID); othersStillActive {
		return fmt.Errorf("%w: transaction %s is still in flight for another user", wdk.ErrNotAbortableAction, txID)
	}

	note := history.NewBuilder().AbortBeforeBroadcast(wdk.ProvenTxStatusInvalid)

	parked, err := a.knownTxRepo.ParkUnbroadcastKnownTx(ctx, txID, []history.Builder{note})
	if err != nil {
		return fmt.Errorf("failed to park known tx %s for abort: %w", txID, err)
	}
	if !parked {
		return fmt.Errorf("%w: transaction %s has broadcast evidence", wdk.ErrNotAbortableAction, txID)
	}

	return nil
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

	// NOTE: broadcast evidence is NOT checked here. It is checked - and acted upon - by the
	// guarded KnownTx park inside abortTx, which is a single atomic write instead of a
	// read-then-write, and which additionally blocks any later claim for a post.

	logger.DebugContext(ctx, "Checking if transaction outputs are unspent")
	if err := a.outputsRepo.ShouldTxOutputsBeUnspent(ctx, txEntity.ID); err != nil {
		return fmt.Errorf("cannot abort transaction with spent outputs: %w", err)
	}

	logger.DebugContext(ctx, "Transaction validation passed")
	return nil
}

func validateTxStatusForAbort(txStatus wdk.TxStatus) error {
	switch txStatus {
	case wdk.TxStatusCompleted, wdk.TxStatusFailed, wdk.TxStatusAborted, wdk.TxStatusSending, wdk.TxStatusUnproven:
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

// txStatusesAbortableBeforeBroadcast are the user-transaction statuses an abort may act on.
// 'sending' is deliberately absent: it means the transaction is queued for (or already went
// through) a post attempt whose outcome is unknown.
var txStatusesAbortableBeforeBroadcast = []wdk.TxStatus{
	wdk.TxStatusUnsigned,
	wdk.TxStatusUnprocessed,
	wdk.TxStatusNoSend,
	wdk.TxStatusNonFinal,
	wdk.TxStatusUnfail,
}

// splitTxsForPreBroadcastAbort returns the ids this call may abort and reports whether any
// other row for the same txID (another user, or a row that already advanced) is still live.
func (a *abortAction) splitTxsForPreBroadcastAbort(transactions []*pkgentity.Transaction, userScope *int) (toAbort []uint, othersStillActive bool) {
	for _, tx := range transactions {
		inScope := userScope == nil || tx.UserID == *userScope
		abortable := slices.Contains(txStatusesAbortableBeforeBroadcast, tx.Status)

		switch {
		case inScope && abortable:
			toAbort = append(toAbort, tx.ID)
		case tx.Status == wdk.TxStatusAborted || tx.Status == wdk.TxStatusFailed:
			// already gone, nothing keeps the tx alive
		default:
			othersStillActive = true
		}
	}

	return toAbort, othersStillActive
}
