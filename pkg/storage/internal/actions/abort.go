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

	// NOTE (residual TOCTOU, deferred to W3a-2 per Decision Record v1): this gate reads
	// KnownTx status outside the abort UoW, so a concurrent ProcessAction can flip KnownTx
	// to 'sending' between this check and the abort's commit. The Transaction-status CAS in
	// abortTx (UpdateTransactionStatusByID against the pre-abort abortable statuses) shrinks
	// the race window to milliseconds - it does not close it. Full closure would require a
	// row lease or a FOR UPDATE re-check of KnownTx inside the same UoW as the abort write.
	logger.DebugContext(ctx, "Checking shared KnownTx for broadcast/network evidence")
	if txEntity.TxID != nil {
		statuses, err := a.knownTxRepo.FindKnownTxStatuses(ctx, *txEntity.TxID)
		if err != nil {
			return fmt.Errorf("failed to check broadcast evidence for abort: %w", err)
		}
		if status, ok := statuses[*txEntity.TxID]; ok && !abortableKnownTxStatus(status) {
			return fmt.Errorf("%w: transaction %s has broadcast/network evidence (known status %q)", wdk.ErrNotAbortableAction, *txEntity.TxID, status)
		}
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

// abortableKnownTxStatus reports whether the shared KnownTx carries no
// broadcast or network-acceptance evidence (P4: abort is an input release).
func abortableKnownTxStatus(status wdk.ProvenTxReqStatus) bool {
	//nolint:exhaustive // default case handles remaining statuses (all refused)
	switch status {
	case wdk.ProvenTxStatusUnprocessed, wdk.ProvenTxStatusNoSend,
		wdk.ProvenTxStatusNonFinal, wdk.ProvenTxStatusUnknown:
		return true
	default:
		return false
	}
}

// txStatusesAbortableBeforeBroadcast are the user-transaction statuses a pre-broadcast
// abort may act on. 'sending' is deliberately absent: it means the transaction is queued
// for (or already went through) a post attempt whose outcome is unknown.
var txStatusesAbortableBeforeBroadcast = []wdk.TxStatus{
	wdk.TxStatusUnsigned,
	wdk.TxStatusUnprocessed,
	wdk.TxStatusNoSend,
	wdk.TxStatusNonFinal,
	wdk.TxStatusUnfail,
}

// AbortUnbroadcastTx releases the inputs of a transaction that already carries a txid (it
// passed SpendTransaction) but provably never reached a broadcaster. The shared KnownTx is
// parked FIRST: that guarded transition is the authority for the whole abort - it only
// applies while the transaction was never posted, and once applied no pipeline can pick the
// transaction up again. Only rows owned by userID are aborted, and the abort backs off
// entirely when any other row keeps the transaction alive.
func (a *abortAction) AbortUnbroadcastTx(ctx context.Context, userID int, txID string) error {
	logger := a.logger.With(slog.String("txID", txID))

	transactions, err := a.transactionsRepo.FindTransactions(ctx, &pkgentity.TransactionReadSpecification{
		TxID: &pkgentity.Comparable[string]{Value: txID, Cmp: pkgentity.Equal},
	})
	if err != nil {
		return fmt.Errorf("failed to find transactions for txID %s: %w", txID, err)
	}

	toAbort, othersStillActive := a.splitTxsForPreBroadcastAbort(transactions, &userID)
	if len(toAbort) == 0 {
		logger.DebugContext(ctx, "skipping abort before broadcast: no transaction in an abortable status")
		return nil
	}

	// Another owner still has this transaction in flight, so the shared KnownTx must stay
	// broadcastable - and then our inputs must stay spent, or that broadcast becomes a
	// double spend.
	if othersStillActive {
		logger.WarnContext(ctx, "skipping abort before broadcast: transaction is still active for another user")
		return nil
	}

	// Park the shared KnownTx FIRST. It is the guard that decides whether this abort may
	// happen at all: it only applies while the tx provably never reached a broadcaster, and
	// once applied no pipeline (send_waiting, background broadcaster re-queue) can pick the
	// tx up again.
	note := history.NewBuilder().AbortBeforeBroadcast(wdk.ProvenTxStatusInvalid)

	parked, err := a.knownTxRepo.ParkUnbroadcastKnownTx(ctx, txID, []history.Builder{note})
	if err != nil {
		return fmt.Errorf("failed to park known tx %s before abort: %w", txID, err)
	}
	if !parked {
		logger.WarnContext(ctx, "skipping abort before broadcast: known tx may have already been handed to a broadcaster")
		return nil
	}

	for _, id := range toAbort {
		if checkErr := a.outputsRepo.ShouldTxOutputsBeUnspent(ctx, id); checkErr != nil {
			a.logger.WarnContext(ctx, "skipping abort: tx outputs already spent", logging.Number("transactionID", id), logging.Error(checkErr))
			continue
		}

		if uowErr := a.uow.Do(ctx, func(txCtx context.Context, repos Providers) error {
			if err := repos.UTXORepo().UnreserveUTXOsByTransactionID(txCtx, id); err != nil {
				return fmt.Errorf("failed to unreserve UTXOs: %w", err)
			}
			if err := repos.OutputRepo().RecreateSpentOutputs(txCtx, id); err != nil {
				return fmt.Errorf("failed to recreate spent outputs: %w", err)
			}
			if err := repos.OutputRepo().MarkCreatedOutputsAsNotSpendable(txCtx, id); err != nil {
				return fmt.Errorf("failed to mark created outputs as not spendable: %w", err)
			}
			// Positive CAS: only abort a row that is still in a pre-broadcast status. A row
			// that advanced concurrently wins and rolls this unit of work back.
			if err := repos.TransactionsRepo().UpdateTransactionStatusByID(txCtx, id, wdk.TxStatusAborted, txStatusesAbortableBeforeBroadcast...); err != nil {
				return fmt.Errorf("failed to update transaction status to aborted: %w", err)
			}
			return nil
		}); uowErr != nil {
			return fmt.Errorf("failed to abort transaction %d before broadcast: %w", id, uowErr)
		}
	}

	logger.InfoContext(ctx, "aborted transaction before broadcast", slog.Int("abortedRows", len(toAbort)))
	return nil
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
