package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seq2"
	"github.com/go-softwarelab/common/pkg/to"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	pkgerrors "github.com/bsv-blockchain/go-wallet-toolbox/pkg/errors"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/service"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

const transactionBatchLength = 16

type process struct {
	logger                *slog.Logger
	commissionCfg         defs.Commission
	txRepo                TransactionsRepo
	outputRepo            OutputRepo
	knownTxRepo           KnownTxRepo
	commissionRepo        CommissionRepo
	utxoRepo              UTXORepo
	services              wdk.Services
	backgroundBroadcaster *service.BackgroundBroadcaster
	randomizer            wdk.Randomizer
	sendWaitingLock       sync.Mutex
	beefVerifier          wdk.BeefVerifier
	scriptsVerifier       wdk.ScriptsVerifier
	uow                   UnitOfWork
}

func newProcessAction(
	ctx context.Context,
	logger *slog.Logger,
	txRepo TransactionsRepo,
	commissionCfg defs.Commission,
	outputRepo OutputRepo,
	knownTxRepo KnownTxRepo,
	commissionRepo CommissionRepo,
	utxoRepo UTXORepo,
	uow UnitOfWork,
	services wdk.Services,
	randomizer wdk.Randomizer,
	beefVerifier wdk.BeefVerifier,
	scriptsVerifier wdk.ScriptsVerifier,
	txBroadcastedChannel chan<- wdk.CurrentTxStatus,
	broadcasterSizing service.Sizing,
) *process {
	logger = logging.Child(logger, "processAction")
	p := &process{
		logger:          logger,
		commissionCfg:   commissionCfg,
		txRepo:          txRepo,
		outputRepo:      outputRepo,
		knownTxRepo:     knownTxRepo,
		commissionRepo:  commissionRepo,
		utxoRepo:        utxoRepo,
		uow:             uow,
		services:        services,
		randomizer:      randomizer,
		beefVerifier:    beefVerifier,
		scriptsVerifier: scriptsVerifier,
	}

	p.backgroundBroadcaster = service.NewBackgroundBroadcaster(ctx, logger, p, txBroadcastedChannel, broadcasterSizing)
	p.backgroundBroadcaster.Start()
	return p
}

func (p *process) Process(ctx context.Context, userID int, args *wdk.ProcessActionArgs) (*wdk.ProcessActionResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "StorageActions-Process", attribute.Int("userID", userID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	logger := p.logger.With(logging.UserID(userID))

	logger.InfoContext(
		ctx, "Starting Process Action",
		slog.Bool("isNewTx", args.IsNewTx),
		slog.Bool("isNoSend", args.IsNoSend),
		slog.Bool("isDelayed", args.IsDelayed),
		slog.Int("sendWithCount", len(args.SendWith)),
	)

	if args.IsNewTx {
		logger.DebugContext(
			ctx, "Processing new transaction",
			slog.String("txID", string(to.Value(args.TxID))),
			slog.String("reference", to.Value(args.Reference)),
			slog.Int("rawTxSize", len(args.RawTx)),
		)
		if err = p.processNewTx(ctx, userID, args); err != nil {
			return nil, err
		}
	}

	if args.IsNoSend && len(args.SendWith) == 0 {
		logger.DebugContext(
			ctx, "NoSend mode - skipping broadcast",
			slog.String("txID", string(to.Value(args.TxID))),
		)
		// NOTE: SendWith overrides IsNoSend, so if SendWith is NOT empty, we will broadcast txs anyway
		return &wdk.ProcessActionResult{}, nil
	}

	txIDs := p.txIDsToBroadcast(args)

	logger.DebugContext(
		ctx, "Preparing transactions for broadcast",
		slog.Int("txIDsCount", len(txIDs)),
		slog.Bool("isDelayed", args.IsDelayed),
	)

	if len(txIDs) > 1 {
		logger.DebugContext(
			ctx, "Setting batch for multiple transactions",
			slog.Int("batchSize", len(txIDs)),
		)

		if err = p.setBatchForTxs(ctx, txIDs); err != nil {
			return nil, fmt.Errorf("failed to set batch for transactions: %w", err)
		}
	}

	logger.DebugContext(
		ctx, "Broadcasting transactions",
		slog.Int("txIDsCount", len(txIDs)),
		slog.Bool("isDelayed", args.IsDelayed),
	)

	result, err := p.broadcastTxs(ctx, txIDs, args.IsDelayed)
	if err != nil {
		return nil, err
	}

	logger.InfoContext(
		ctx, "Process Action completed",
		slog.Int("sendWithResultsCount", len(result.SendWithResults)),
		slog.Int("notDelayedResultsCount", len(result.NotDelayedResults)),
		slog.Bool("isDelayed", args.IsDelayed),
	)

	return result, nil
}

func (p *process) txIDsToBroadcast(args *wdk.ProcessActionArgs) []string {
	count := len(args.SendWith)
	if args.TxID != nil {
		count++
	}

	result := make([]string, 0, count)
	for _, txID := range args.SendWith {
		result = append(result, string(txID))
	}
	if args.TxID != nil {
		result = append(result, string(*args.TxID))
	}

	return result
}

func (p *process) processNewTx(ctx context.Context, userID int, args *wdk.ProcessActionArgs) error {
	txlogger := p.logger.With(
		logging.UserID(userID),
		slog.String("reference", to.Value(args.Reference)),
	)

	txlogger.DebugContext(
		ctx, "Building transaction from raw bytes",
		slog.Int("rawTxSize", len(args.RawTx)),
	)

	tx, err := transaction.NewTransactionFromBytes(args.RawTx)
	if err != nil {
		return fmt.Errorf("failed to build transaction object from raw tx bytes: %w", err)
	}

	txID := tx.TxID().String()
	if txID != string(*args.TxID) {
		return fmt.Errorf("txID mismatch: provided %s, calculated from raw tx: %s", *args.TxID, txID)
	}

	txlogger.DebugContext(
		ctx, "Checking nLockTime finality",
		slog.String("txID", txID),
	)

	isFinal, err := p.services.NLockTimeIsFinal(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to check nLockTime finality: %w", err)
	}
	if !isFinal {
		return fmt.Errorf("transaction nLockTime is not final")
	}

	txlogger.DebugContext(ctx, "Finding transaction by reference")

	txEntity, err := p.txRepo.FindTransactionByReference(ctx, userID, *args.Reference)
	if err != nil {
		return fmt.Errorf("failed to find transaction by reference: %w", err)
	}

	err = p.validateStateOfTableTx(*args.Reference, txEntity)
	if err != nil {
		return err
	}

	txlogger.DebugContext(
		ctx, "Finding outputs for transaction validation",
		slog.String("txID", txID),
		slog.Uint64("transactionID", uint64(txEntity.ID)),
	)

	outputs, err := p.outputRepo.FindOutputsByTransactionID(ctx, txEntity.ID)
	if err != nil {
		return fmt.Errorf("failed to find inputs and outputs of transaction: %w", err)
	}

	txlogger.DebugContext(
		ctx, "Validating transaction outputs",
		slog.String("txID", txID),
		slog.Int("outputsCount", len(outputs)),
	)

	err = p.validateNewTxOutputs(tx, outputs)
	if err != nil {
		return err
	}

	if p.commissionCfg.Satoshis > 0 {
		txlogger.DebugContext(
			ctx, "Validating commission",
			slog.String("txID", txID),
			slog.Uint64("transactionID", uint64(txEntity.ID)),
			slog.Uint64("commissionSatoshis", p.commissionCfg.Satoshis),
		)

		if err = p.validateCommission(ctx, userID, txEntity.ID, outputs); err != nil {
			return fmt.Errorf("commission validation failed: %w", err)
		}
	}

	// TODO: Add db transactionID to KnownTx.Notify

	// TODO: Remove too long locking scripts (len > storage.maxOutputScript)

	newTxStatus, newReqStatus := p.newStatuses(args)

	txlogger.DebugContext(
		ctx, "Updating transaction status and raw data",
		slog.String("txID", txID),
		slog.String("newTxStatus", string(newTxStatus)),
		slog.String("newReqStatus", string(newReqStatus)),
	)

	err = p.txRepo.SpendTransaction(ctx, entity.UpdatedTx{
		UserID:        userID,
		TransactionID: txEntity.ID,
		TxID:          txID,
		TxStatus:      newTxStatus,
		ReqTxStatus:   newReqStatus,
		RawTx:         args.RawTx,
		InputBeef:     txEntity.InputBEEF,
		Tx:            tx,
	}, history.NewBuilder().ProcessAction(userID))
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	return nil
}

func (p *process) validateStateOfTableTx(reference string, tableTx *pkgentity.Transaction) error {
	if tableTx == nil {
		return fmt.Errorf("transaction with reference (%s) not found in the database", reference)
	}

	if !tableTx.IsOutgoing {
		return fmt.Errorf("transaction with reference (%s) is not outgoing", reference)
	}

	if len(tableTx.InputBEEF) == 0 {
		return fmt.Errorf("transaction with reference (%s) has no inputBEEF. This suggests the transaction may have already been processed. Try with (IsNewTx = false)", reference)
	}

	if tableTx.Status != wdk.TxStatusUnsigned && tableTx.Status != wdk.TxStatusUnprocessed {
		return fmt.Errorf("transaction with reference (%s) is not in a valid status for processing", reference)
	}

	return nil
}

func (p *process) validateNewTxOutputs(tx *transaction.Transaction, outputs []*pkgentity.Output) error {
	for _, output := range outputs {
		if output.Change {
			continue
		}

		if output.LockingScript == nil {
			return fmt.Errorf("locking script is nil for output %d", output.ID)
		}

		voutInt := must.ConvertToIntFromUnsigned(output.Vout)
		if voutInt >= len(tx.Outputs) {
			return fmt.Errorf("output index %d is out of range of provided tx outputs count %d", voutInt, len(tx.Outputs))
		}

		fromDB := output.LockingScript
		providedInArgs := tx.Outputs[voutInt].LockingScript.Bytes()
		if !bytes.Equal(providedInArgs, fromDB) {
			return fmt.Errorf("locking script mismatch at vout: %d, provided %x, calculated from raw tx: %x", voutInt, providedInArgs, fromDB)
		}
	}
	return nil
}

func (p *process) validateCommission(ctx context.Context, userID int, transactionID uint, outputs []*pkgentity.Output) error {
	commissionEntity, err := p.commissionRepo.FindCommission(ctx, userID, transactionID)
	if err != nil {
		return fmt.Errorf("failed to find commission for user %d and transaction %d: %w", userID, transactionID, err)
	}

	if commissionEntity == nil {
		return fmt.Errorf("commission not found for user %d and transaction %d", userID, transactionID)
	}

	if len(commissionEntity.LockingScript) == 0 {
		return fmt.Errorf("commission locking script is empty for user %d and transaction %d", userID, transactionID)
	}

	includesCommissionOutput := seq.Exists(
		seq.FromSlice(outputs),
		func(output *pkgentity.Output) bool {
			return satoshi.MustEqual(output.Satoshis, commissionEntity.Satoshis) &&
				output.LockingScript != nil &&
				bytes.Equal(output.LockingScript, commissionEntity.LockingScript)
		},
	)

	if !includesCommissionOutput {
		return fmt.Errorf("transaction %d did not include an output to cover service fee", transactionID)
	}

	return nil
}

func (p *process) newStatuses(args *wdk.ProcessActionArgs) (txStatus wdk.TxStatus, reqStatus wdk.ProvenTxReqStatus) {
	switch {
	case args.IsNoSend:
		reqStatus = wdk.ProvenTxStatusNoSend
		txStatus = wdk.TxStatusNoSend
	case args.IsDelayed:
		reqStatus = wdk.ProvenTxStatusUnsent
		txStatus = wdk.TxStatusUnprocessed
	default:
		reqStatus = wdk.ProvenTxStatusUnprocessed
		txStatus = wdk.TxStatusUnprocessed
	}

	return txStatus, reqStatus
}

func (p *process) broadcastTxs(ctx context.Context, txIDs []string, isDelayed bool) (*wdk.ProcessActionResult, error) {
	logger := p.logger.With(
		slog.Int("txIDsCount", len(txIDs)),
		slog.Bool("isDelayed", isDelayed),
	)

	knownTxStatusesLookup, err := p.getKnownTxStatuses(ctx, txIDs...)
	if err != nil {
		return nil, err
	}

	txReferencesLookup, err := p.txRepo.FindReferencesByTxIDs(ctx, txIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to find references for txIDs: %w", err)
	}

	txLabelsLookup, err := p.txRepo.GetLabelsForTxIDs(ctx, txIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to find labels for txIDs: %w", err)
	}

	sendWithResults := make([]wdk.SendWithResult, 0, len(txIDs))
	notDelayedResults := make([]wdk.ReviewActionResult, 0, to.IfThen(!isDelayed, len(txIDs)).ElseThen(0))
	var readyToSendTxIDs []string

	logger.DebugContext(ctx, "Categorizing transactions by status")

	for _, txID := range txIDs {
		currentStatus, ok := knownTxStatusesLookup[txID]
		if !ok {
			return nil, fmt.Errorf("transaction status not found for txID %s", txID)
		}

		if currentStatus.AlreadySent() {
			logger.DebugContext(
				ctx, "Transaction already sent - adding to results",
				slog.String("txID", txID),
				slog.String("status", string(currentStatus)),
			)

			sendWithResults = append(sendWithResults, wdk.SendWithResult{
				TxID:   primitives.TXIDHexString(txID),
				Status: currentStatus.SendWithResultStatus(),
			})

			logger.DebugContext(
				ctx, "Creating spendable UTXOs",
				slog.String("txID", txID),
			)

			err = p.utxoRepo.CreateUTXOForSpendableOutputsByTxID(ctx, txID)
			if err != nil {
				return nil, fmt.Errorf("failed to make outputs spendable for txID %s: %w", txID, err)
			}
		} else {
			logger.DebugContext(
				ctx, "Transaction ready to send",
				slog.String("txID", txID),
				slog.String("status", string(currentStatus)),
			)
			readyToSendTxIDs = append(readyToSendTxIDs, txID)
		}
	}

	if len(sendWithResults) == len(txIDs) {
		logger.DebugContext(
			ctx, "All transactions already broadcasted - returning early",
			slog.Int("alreadySentCount", len(sendWithResults)),
		)
		// All txs are already broadcasted, so we return the results without sending them again
		return &wdk.ProcessActionResult{
			SendWithResults: sendWithResults,
		}, nil
	}

	if len(readyToSendTxIDs) == 0 {
		// This should never happen, because:
		// 1. When all txs are already broadcasted, we return early.
		// 2. If there are txs with other-then-unproven statuses, they should be in the readyToSendTxIDs.
		// So, if we reach this point, it means that the transactions have unsupported broadcast statuses.
		return nil, fmt.Errorf("unsupported broadcast status for all txs: %v", knownTxStatusesLookup)
	}

	logger.DebugContext(
		ctx, "Building BEEF for ready-to-send transactions",
		slog.Int("readyToSendCount", len(readyToSendTxIDs)),
	)

	// Direct sources only: this BEEF feeds script verification and EF
	// construction, both of which need just each input's source output. A full
	// ancestry build re-validated every ancestor BUMP root per transaction —
	// at high TPS that was the single largest CPU cost in the storage server
	// (shared fuel parents re-validated for every spend).
	beef, err := p.knownTxRepo.GetBEEFForTxIDs(ctx, seq.FromSlice(readyToSendTxIDs),
		entity.WithStatusesToFilterOut(wdk.ProvenTxReqProblematicStatuses...),
		entity.WithDirectSourcesOnly(),
	)
	if err != nil {
		p.abortTxsBeforeBroadcast(ctx, readyToSendTxIDs)
		return nil, fmt.Errorf("failed to build valid BEEF: %w", err)
	}

	logger.DebugContext(
		ctx, "Verifying built EF",
		slog.Int("readyToSendCount", len(readyToSendTxIDs)),
	)

	// Hydrate only the subject txs' direct inputs: the direct-sources-only
	// BEEF deliberately omits grandparents, and neither script verification
	// nor EF construction needs them.
	if err = txutils.HydrateBEEFSubjects(beef, readyToSendTxIDs); err != nil {
		p.abortTxsBeforeBroadcast(ctx, readyToSendTxIDs)
		return nil, fmt.Errorf("failed to hydrate beef for script verification: %w", err)
	}

	// Verify scripts for each transaction before broadcasting.
	// We use scripts-only verification (not full BEEF verification) because:
	// 1. We're sending EF format to ARC, not BEEF
	// 2. Merkle path validation can fail during block reorgs
	// 3. Input BEEF was validated when creating action with external inputs or during internalization, so there's no need to validate it again
	for _, txID := range readyToSendTxIDs {
		tx := beef.FindTransaction(txID)
		if tx == nil {
			p.abortTxsBeforeBroadcast(ctx, readyToSendTxIDs)
			return nil, fmt.Errorf("transaction %s not found in beef", txID)
		}

		var ok bool
		ok, err = p.scriptsVerifier.VerifyScripts(ctx, tx)
		if err != nil {
			p.abortTxsBeforeBroadcast(ctx, readyToSendTxIDs)
			return nil, fmt.Errorf("failed to verify scripts for tx %s: %w", txID, err)
		}
		if !ok {
			p.abortTxsBeforeBroadcast(ctx, readyToSendTxIDs)
			return nil, fmt.Errorf("scripts validation failed for tx %s", txID)
		}
	}

	if isDelayed {
		// NOTE: the delayed path does NOT bump attempts here. The tx is only being QUEUED for a
		// later broadcast; the attempt is counted when it is actually posted (see BackgroundBroadcast).
		logger.DebugContext(
			ctx, "Processing delayed transactions",
			slog.Int("readyToSendCount", len(readyToSendTxIDs)),
		)

		var resultsForDelayedTxs []wdk.SendWithResult
		resultsForDelayedTxs, err = p.processDelayedTransactions(ctx, readyToSendTxIDs, beef)
		if err != nil {
			return nil, fmt.Errorf("failed to process delayed transactions: %w", err)
		}

		sendWithResults = append(sendWithResults, resultsForDelayedTxs...)

		return &wdk.ProcessActionResult{
			SendWithResults: sendWithResults,
		}, nil
	}

	logger.DebugContext(
		ctx, "Marking transactions as submitting before broadcast",
		slog.Int("readyToSendCount", len(readyToSendTxIDs)),
	)

	if err = p.knownTxRepo.MarkKnownTxsAsSubmitting(ctx, readyToSendTxIDs); err != nil {
		p.abortTxsBeforeBroadcast(ctx, readyToSendTxIDs)
		return nil, fmt.Errorf("failed to mark txs as submitting: %w", err)
	}

	// Count the attempt only for the txs we are ACTUALLY posting now (readyToSendTxIDs), right
	// before the post. Already-sent txs (which took the early return / sendWithResults branch) and
	// delayed/queued txs are intentionally NOT bumped here, so attempts reflect real broadcast
	// attempts and proof-timeout logic fires on true post counts.
	logger.DebugContext(
		ctx, "Increasing attempt counters for transactions being posted",
		slog.Int("readyToSendCount", len(readyToSendTxIDs)),
	)

	if err = p.knownTxRepo.IncreaseKnownTxAttemptsForTxIDs(ctx, readyToSendTxIDs); err != nil {
		p.abortTxsBeforeBroadcast(ctx, readyToSendTxIDs)
		return nil, fmt.Errorf("failed to increase known tx attempts: %w", err)
	}

	logger.DebugContext(
		ctx, "Posting BEEF to services",
		slog.Int("readyToSendCount", len(readyToSendTxIDs)),
	)

	results, err := p.services.PostFromBEEF(ctx, beef, readyToSendTxIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to post BEEF: %w", err)
	}

	var (
		sendWithResult     wdk.SendWithResult
		reviewActionResult wdk.ReviewActionResult
	)

	aggregated := results.Aggregated(txIDs)
	p.confirmDoubleSpends(ctx, aggregated)

	logger.DebugContext(
		ctx, "Processing individual transaction results",
		slog.Int("aggregatedResultsCount", len(aggregated)),
		slog.Int("readyToSendCount", len(readyToSendTxIDs)),
	)

	for _, broadcastedTxID := range readyToSendTxIDs {
		aggBroadcastResult, ok := aggregated[broadcastedTxID]
		if !ok {
			logger.DebugContext(
				ctx, "No broadcast result found for transaction - using failed result",
				slog.String("txID", broadcastedTxID),
			)
			sendWithResult, reviewActionResult = p.failedResultForTxID(broadcastedTxID)
		} else {
			logger.DebugContext(
				ctx, "Processing broadcast result for transaction",
				slog.String("txID", broadcastedTxID),
				slog.String("aggregatedStatus", string(aggBroadcastResult.Status)),
			)

			sendWithResult, reviewActionResult, err = p.updateSingleTx(
				ctx,
				broadcastedTxID,
				aggBroadcastResult,
				results.ServiceErrors(),
				beef,
				readyToSendTxIDs,
				txReferencesLookup[broadcastedTxID],
				txLabelsLookup[broadcastedTxID],
			)
			if err != nil {
				return nil, fmt.Errorf(
					"cannot update single tx after broadcast: %w",
					pkgerrors.NewProcessActionError(sendWithResults, notDelayedResults).
						Wrap(pkgerrors.NewTransactionErrorFromTxIDHex(broadcastedTxID)),
				)
			}
		}

		logger.DebugContext(
			ctx, "BroadcastTxs completed successfully",
			slog.Int("totalSendWithResults", len(sendWithResults)),
			slog.Int("totalNotDelayedResults", len(notDelayedResults)),
		)

		sendWithResults = append(sendWithResults, sendWithResult)
		notDelayedResults = append(notDelayedResults, reviewActionResult)
	}

	return &wdk.ProcessActionResult{
		SendWithResults:   sendWithResults,
		NotDelayedResults: notDelayedResults,
	}, nil
}

func (p *process) setBatchForTxs(ctx context.Context, txIDs []string) error {
	batch, err := p.randomizer.Base64(transactionBatchLength)
	if err != nil {
		return fmt.Errorf("failed to generate random batch: %w", err)
	}

	err = p.knownTxRepo.SetBatchForKnownTxs(ctx, txIDs, batch)
	if err != nil {
		return fmt.Errorf("failed to set batch for txIDs: %w", err)
	}

	return nil
}

// queueTxForDelayedBroadcast moves one transaction into the pre-send state and
// makes its change claimable. A status update that is skipped because the
// transaction already advanced past this stage is expected and not an error.
func (p *process) queueTxForDelayedBroadcast(ctx context.Context, txID string) error {
	err := p.knownTxRepo.UpdateKnownTxStatus(ctx, txID, wdk.ProvenTxStatusUnsent, wdk.ProvenTxReqBeyondBroadcastStageStatuses, nil)
	if err != nil && !errors.Is(err, repo.ErrStatusUpdateSkipped) {
		return fmt.Errorf("failed to update known tx status for txID %s: %w", txID, err)
	}
	if err != nil {
		// Legitimate: the tx is already beyond the broadcast stage, so there is
		// nothing to re-queue as unsent.
		p.logger.DebugContext(ctx, "known tx status update skipped (already beyond broadcast stage)", slog.String("txID", txID), logging.Error(err))
	}

	err = p.txRepo.UpdateTransactionStatusByTxID(ctx, txID, wdk.TxStatusSending, wdk.TxStatusUnprocessed)
	if err != nil && !errors.Is(err, repo.ErrStatusUpdateSkipped) {
		return fmt.Errorf("failed to update transaction status for txID %s: %w", txID, err)
	}
	if err != nil {
		// The user transaction is no longer in the expected pre-send state
		// (e.g. it advanced concurrently). Do not downgrade it.
		p.logger.DebugContext(ctx, "transaction status update skipped (not in expected pre-send state)", slog.String("txID", txID), logging.Error(err))
	}

	// Make this tx's change/fuel outputs claimable NOW at the "sending" tier.
	// Waiting for network acceptance to flip them spendable starves the fuel
	// pool at high request rates, because the acceptance pipeline runs orders
	// of magnitude slower than creation. Only spend policies that accept
	// unconfirmed funds will select them.
	if err = p.utxoRepo.MakeChangeSpendableAndIndexByTxID(ctx, txID); err != nil {
		return fmt.Errorf("failed to make outputs spendable for delayed txID %s: %w", txID, err)
	}
	return nil
}

func (p *process) processDelayedTransactions(ctx context.Context, txIDs []string, beef *transaction.Beef) ([]wdk.SendWithResult, error) {
	sendWithResults := make([]wdk.SendWithResult, 0, len(txIDs))
	for _, txID := range txIDs {
		if err := p.queueTxForDelayedBroadcast(ctx, txID); err != nil {
			return nil, err
		}

		sendWithResults = append(sendWithResults, wdk.SendWithResult{
			TxID:   primitives.TXIDHexString(txID),
			Status: wdk.SendWithResultStatusSending,
		})
	}

	added := p.backgroundBroadcaster.Add(beef, txIDs)
	if !added {
		p.logger.DebugContext(ctx, "Background broadcaster channel is full, will be added later by the CRON")
	}

	return sendWithResults, nil
}

func (p *process) updateSingleTx(
	ctx context.Context,
	txID string,
	aggBroadcastResult *wdk.AggregatedPostedTxID,
	serviceErrors map[string]error,
	beef *transaction.Beef,
	txIDs []string,
	reference string,
	labels []string,
) (
	sendWithResult wdk.SendWithResult,
	reviewActionResult wdk.ReviewActionResult,
	err error,
) {
	var (
		newReqStatus wdk.ProvenTxReqStatus
		newTxStatus  wdk.TxStatus
		spendable    bool
	)

	newReqStatus, newTxStatus, spendable, reviewActionResult, sendWithResult, err = p.singleTxBroadcastResult(aggBroadcastResult, txID, serviceErrors, reference, labels)
	if err != nil {
		return sendWithResult, reviewActionResult, err
	}

	notes := p.notesForPostBEEF(newReqStatus, aggBroadcastResult, serviceErrors, beef, txIDs)

	err = p.uow.Do(ctx, func(txCtx context.Context, repos Providers) error {
		var uowErr error
		// Guarded Transaction write FIRST. A broadcast result may only transition a tx that is
		// still in a broadcast-eligible state: unprocessed (initial), sending (in-flight),
		// nosend (broadcast via SendWith), or unproven (re-broadcast while awaiting proof, e.g.
		// SendWaitingTransactions). If it matches zero rows the tx is already proven/mined
		// (completed) or terminal (failed) — a late/raced result. Warn and no-op BEFORE the
		// failure cascade or the KnownTx downgrade, so we never downgrade a proven tx or
		// re-release inputs the proof already proved.
		uowErr = repos.TransactionsRepo().UpdateTransactionStatusByTxID(txCtx, txID, newTxStatus,
			wdk.TxStatusUnprocessed, wdk.TxStatusSending, wdk.TxStatusNoSend, wdk.TxStatusUnproven)
		if uowErr != nil {
			if errors.Is(uowErr, repo.ErrStatusUpdateSkipped) {
				p.logger.WarnContext(txCtx, "ignoring late broadcast result for transaction beyond broadcast stage",
					slog.String("txID", txID), slog.String("attemptedStatus", string(newTxStatus)), logging.Error(uowErr))
				return nil
			}
			return fmt.Errorf("failed to update transaction status after broadcast: %w", uowErr)
		}

		if newTxStatus == wdk.TxStatusFailed {
			var transactionIDs []uint
			transactionIDs, uowErr = repos.TransactionsRepo().FindTransactionIDsByTxID(txCtx, txID)
			if uowErr != nil {
				return fmt.Errorf("failed to find transaction IDs for failed tx %s: %w", txID, uowErr)
			}
			for _, id := range transactionIDs {
				if uowErr = repos.OutputRepo().RecreateSpentOutputs(txCtx, id); uowErr != nil {
					return fmt.Errorf("failed to restore spent outputs for failed tx %s: %w", txID, uowErr)
				}
				if uowErr = repos.OutputRepo().MarkCreatedOutputsAsNotSpendable(txCtx, id); uowErr != nil {
					return fmt.Errorf("failed to mark created outputs as not spendable for failed tx %s: %w", txID, uowErr)
				}
			}
		}

		uowErr = repos.KnownTxRepo().UpdateKnownTxStatus(txCtx, txID, newReqStatus, wdk.ProvenTxReqBeyondBroadcastStageStatuses, notes)
		if uowErr != nil {
			if errors.Is(uowErr, repo.ErrStatusUpdateSkipped) {
				// The shared KnownTx is already beyond the broadcast stage; leaving it as-is
				// is legitimate. The Transaction write above already matched, so continue.
				p.logger.DebugContext(txCtx, "known tx status update skipped (beyond broadcast stage)", slog.String("txID", txID), logging.Error(uowErr))
			} else {
				return fmt.Errorf("failed to update known tx status after broadcast: %w", uowErr)
			}
		}

		if spendable {
			uowErr = repos.UTXORepo().CreateUTXOForSpendableOutputsByTxID(txCtx, txID)
			if uowErr != nil {
				return fmt.Errorf("failed to make outputs spendable after broadcast: %w", uowErr)
			}
		}

		return nil
	})

	return sendWithResult, reviewActionResult, err
}

func (p *process) failedResultForTxID(txID string) (wdk.SendWithResult, wdk.ReviewActionResult) {
	return wdk.SendWithResult{
			TxID:   primitives.TXIDHexString(txID),
			Status: wdk.SendWithResultStatusFailed,
		}, wdk.ReviewActionResult{
			TxID:   primitives.TXIDHexString(txID),
			Status: wdk.ReviewActionResultStatusServiceError,
		}
}

func (p *process) notesForPostBEEF(
	provenTxReqStatus wdk.ProvenTxReqStatus,
	aggBroadcastResult *wdk.AggregatedPostedTxID,
	serviceErrors map[string]error,
	beef *transaction.Beef,
	txIDs []string,
) []history.Builder {
	notesCount := 0
	for _, result := range aggBroadcastResult.TxIDResults {
		notesCount += len(result.Notes)
	}

	records := make([]history.Builder, 0, notesCount+len(serviceErrors)+1)

	if len(serviceErrors) > 0 {
		txData := history.BeefObj(beef)

		sortedErrors := seq2.SortByKeys(seq2.FromMap(serviceErrors))
		errorNotes := seq2.MapTo(sortedErrors, func(serviceName string, err error) history.Builder {
			return history.NewBuilder().PostBeefError(serviceName, txData, txIDs, err.Error())
		})
		_ = slices.AppendSeq(records, errorNotes)
	}

	for _, result := range aggBroadcastResult.TxIDResults {
		for _, note := range result.Notes {
			records = append(records, history.NewBuilderFromNote(note))
		}
	}

	records = append(records, history.NewBuilder().AggregateResults(history.AggregatedBroadcastResult{
		StatusNow:         provenTxReqStatus,
		AggStatus:         aggBroadcastResult.Status,
		SuccessCount:      aggBroadcastResult.SuccessCount,
		DoubleSpendCount:  aggBroadcastResult.DoubleSpendCount,
		StatusErrorCount:  aggBroadcastResult.StatusErrorCount,
		ServiceErrorCount: aggBroadcastResult.ServiceErrorCount,
	}))

	return records
}

func (p *process) getKnownTxStatuses(ctx context.Context, txIDs ...string) (map[string]wdk.ProvenTxReqStatus, error) {
	statuses, err := p.knownTxRepo.FindKnownTxStatuses(ctx, txIDs...)
	if err != nil {
		return nil, fmt.Errorf("failed to find known tx status: %w", err)
	}

	lookup := make(map[string]wdk.ProvenTxReqStatus, len(txIDs))
	for _, txID := range txIDs {
		knownTxStatus, statusFound := statuses[txID]
		if !statusFound {
			return nil, fmt.Errorf("known tx status for txID %s not found", txID)
		}

		if knownTxStatus == wdk.ProvenTxStatusUnfail {
			return nil, fmt.Errorf("wrong statuses to proceed with broadcast: %s", knownTxStatus)
		}

		lookup[txID] = knownTxStatus
	}

	return lookup, nil
}

func (p *process) singleTxBroadcastResult(aggBroadcastResult *wdk.AggregatedPostedTxID, txID string, serviceErrors map[string]error, reference string, labels []string) (
	reqStatus wdk.ProvenTxReqStatus,
	txStatus wdk.TxStatus,
	spendable bool,
	reviewActionResult wdk.ReviewActionResult,
	sendWithResult wdk.SendWithResult,
	err error,
) {
	reviewActionResult = wdk.ReviewActionResult{
		TxID:      primitives.TXIDHexString(txID),
		Errors:    wdk.ReviewActionErrors(serviceErrors),
		Reference: reference,
		Labels:    labels,
	}

	sendWithResult = wdk.SendWithResult{
		TxID: primitives.TXIDHexString(txID),
	}

	switch aggBroadcastResult.Status {
	case wdk.AggregatedPostedTxIDSuccess:
		reqStatus = wdk.ProvenTxStatusUnmined
		txStatus = wdk.TxStatusUnproven
		spendable = true
		sendWithResult.Status = wdk.SendWithResultStatusUnproven
		reviewActionResult.Status = wdk.ReviewActionResultStatusSuccess
	case wdk.AggregatedPostedTxIDDoubleSpend:
		reqStatus = wdk.ProvenTxStatusDoubleSpend
		txStatus = wdk.TxStatusFailed
		spendable = false
		sendWithResult.Status = wdk.SendWithResultStatusFailed
		reviewActionResult.Status = wdk.ReviewActionResultStatusDoubleSpend
		reviewActionResult.CompetingTxs = seq.Collect(maps.Keys(aggBroadcastResult.CompetingTxs))
		// TODO: Build reviewActionResult.CompetingBeef
	case wdk.AggregatedPostedTxIDInvalidTx:
		reqStatus = wdk.ProvenTxStatusInvalid
		txStatus = wdk.TxStatusFailed
		spendable = false
		sendWithResult.Status = wdk.SendWithResultStatusFailed
		reviewActionResult.Status = wdk.ReviewActionResultStatusInvalidTx
	case wdk.AggregatedPostedTxIDServiceError:
		reqStatus = wdk.ProvenTxStatusSending
		txStatus = wdk.TxStatusSending
		// P5 (Decision Record v1): no network evidence -> change must not become
		// spendable; SendWaiting retries and the success path creates the UTXOs.
		spendable = false
		sendWithResult.Status = wdk.SendWithResultStatusSending
		reviewActionResult.Status = wdk.ReviewActionResultStatusServiceError
	default:
		err = fmt.Errorf("unknown AggregatedPostedTxIDStatus %s", aggBroadcastResult.Status)
	}

	return reqStatus, txStatus, spendable, reviewActionResult, sendWithResult, err
}

func (p *process) abortTxsBeforeBroadcast(ctx context.Context, txIDs []string) {
	for _, txID := range txIDs {
		if err := p.abortTxByStringID(ctx, txID); err != nil {
			p.logger.ErrorContext(ctx, "failed to abort tx before broadcast", slog.String("txID", txID), logging.Error(err))
		}
	}
}

func (p *process) abortTxByStringID(ctx context.Context, txID string) error {
	transactionIDs, err := p.txRepo.FindTransactionIDsByTxID(ctx, txID)
	if err != nil {
		return fmt.Errorf("failed to find transaction IDs for txID %s: %w", txID, err)
	}

	for _, id := range transactionIDs {
		if checkErr := p.outputRepo.ShouldTxOutputsBeUnspent(ctx, id); checkErr != nil {
			p.logger.WarnContext(ctx, "skipping abort: tx outputs already spent", logging.Number("transactionID", id), logging.Error(checkErr))
			continue
		}

		if uowErr := p.uow.Do(ctx, func(txCtx context.Context, repos Providers) error {
			if err := repos.UTXORepo().UnreserveUTXOsByTransactionID(txCtx, id); err != nil {
				return fmt.Errorf("failed to unreserve UTXOs: %w", err)
			}
			if err := repos.OutputRepo().RecreateSpentOutputs(txCtx, id); err != nil {
				return fmt.Errorf("failed to recreate spent outputs: %w", err)
			}
			if err := repos.OutputRepo().MarkCreatedOutputsAsNotSpendable(txCtx, id); err != nil {
				return fmt.Errorf("failed to mark created outputs as not spendable: %w", err)
			}
			if err := repos.TransactionsRepo().UpdateTransactionStatusByID(txCtx, id, wdk.TxStatusAborted); err != nil {
				return fmt.Errorf("failed to update transaction status to aborted: %w", err)
			}
			return nil
		}); uowErr != nil {
			return fmt.Errorf("failed to abort transaction %d before broadcast: %w", id, uowErr)
		}
	}
	return nil
}

func (p *process) StopBackgroundBroadcaster() {
	if p.backgroundBroadcaster != nil {
		p.backgroundBroadcaster.Stop()
	}
}

func (p *process) BackgroundBroadcast(ctx context.Context, beef *transaction.Beef, txIDs []string) ([]wdk.ReviewActionResult, error) {
	results, err := p.services.PostFromBEEF(ctx, beef, txIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to post BEEF in background: %w", err)
	}

	txReferencesLookup, err := p.txRepo.FindReferencesByTxIDs(ctx, txIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to find references for txIDs in background broadcast: %w", err)
	}

	txLabelsLookup, err := p.txRepo.GetLabelsForTxIDs(ctx, txIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to find labels for txIDs in background broadcast: %w", err)
	}

	aggregated := results.Aggregated(txIDs)
	p.confirmDoubleSpends(ctx, aggregated)

	bResults := make([]wdk.ReviewActionResult, 0, len(txIDs))
	for _, broadcastedTxID := range txIDs {
		aggBroadcastResult, ok := aggregated[broadcastedTxID]
		if !ok {
			return nil, fmt.Errorf("no broadcast result found for txID %s", broadcastedTxID)
		}

		var reviewActionResult wdk.ReviewActionResult
		_, reviewActionResult, err = p.updateSingleTx(
			ctx,
			broadcastedTxID,
			aggBroadcastResult,
			results.ServiceErrors(),
			beef,
			txIDs,
			txReferencesLookup[broadcastedTxID],
			txLabelsLookup[broadcastedTxID],
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update single tx after background broadcast (txID: %s): %w", broadcastedTxID, err)
		}

		p.logger.DebugContext(ctx, "Background broadcast result", "txID", broadcastedTxID, "status", reviewActionResult.Status)
		bResults = append(bResults, reviewActionResult)
	}

	// Count the attempt only AFTER a completed post round (post + per-tx status/UTXO apply), not
	// before the post. attempts therefore counts completed post rounds. Bumping before the async
	// post would touch the shared known_tx row early and race with concurrent internalize/abort
	// UTXO accounting on MVCC engines (a pre-existing hazard, tracked separately for a later wave).
	// Under-counting on a crash mid-post is the conservative direction: ApplyProofTimeouts then
	// fires later, never earlier. (The foreground broadcastTxs path bumps before its PostFromBEEF
	// because it is synchronous in the caller goroutine and so cannot race this way.)
	if err = p.knownTxRepo.IncreaseKnownTxAttemptsForTxIDs(ctx, txIDs); err != nil {
		return nil, fmt.Errorf("failed to increase known tx attempts in background broadcast: %w", err)
	}

	return bResults, nil
}
