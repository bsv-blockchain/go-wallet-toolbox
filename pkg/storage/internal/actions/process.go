package actions

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	broadcastError "github.com/bsv-blockchain/go-wallet-toolbox/pkg/errors"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/service"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seq2"
	"github.com/go-softwarelab/common/pkg/to"
)

type process struct {
	logger                *slog.Logger
	commissionCfg         defs.Commission
	txRepo                TransactionsRepo
	outputRepo            OutputRepo
	knownTxRepo           KnownTxRepo
	commissionRepo        CommissionRepo
	services              wdk.Services
	backgroundBroadcaster *service.BackgroundBroadcaster
}

func newProcessAction(
	ctx context.Context,
	logger *slog.Logger,
	txRepo TransactionsRepo,
	commissionCfg defs.Commission,
	outputRepo OutputRepo,
	knownTxRepo KnownTxRepo,
	commissionRepo CommissionRepo,
	services wdk.Services,
) *process {
	logger = logging.Child(logger, "processAction")
	p := &process{
		logger:         logger,
		commissionCfg:  commissionCfg,
		txRepo:         txRepo,
		outputRepo:     outputRepo,
		knownTxRepo:    knownTxRepo,
		commissionRepo: commissionRepo,
		services:       services,
	}

	p.backgroundBroadcaster = service.NewBackgroundBroadcaster(ctx, logger, p)
	p.backgroundBroadcaster.Start()
	return p
}

func (p *process) Process(ctx context.Context, userID int, args *wdk.ProcessActionArgs) (*wdk.ProcessActionResult, error) {
	if args.IsNewTx {
		if err := p.processNewTx(ctx, userID, args); err != nil {
			return nil, err
		}
	}

	if args.IsNoSend && len(args.SendWith) == 0 {
		// NOTE: SendWith overrides IsNoSend, so if SendWith is NOT empty, we will broadcast txs anyway
		return &wdk.ProcessActionResult{}, nil
	}

	return p.broadcastTxs(ctx, p.txIDsToBroadcast(args), args.IsDelayed)
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
	tx, err := transaction.NewTransactionFromBytes(args.RawTx)
	if err != nil {
		return fmt.Errorf("failed to build transaction object from raw tx bytes: %w", err)
	}

	txID := tx.TxID().String()
	if txID != string(*args.TxID) {
		return fmt.Errorf("txID mismatch: provided %s, calculated from raw tx: %s", *args.TxID, txID)
	}

	// TODO: Services::nLockTimeIsFinal(tx)

	txEntity, err := p.txRepo.FindTransactionByReference(ctx, userID, *args.Reference)
	if err != nil {
		return fmt.Errorf("failed to find transaction by reference: %w", err)
	}

	err = p.validateStateOfTableTx(*args.Reference, txEntity)
	if err != nil {
		return err
	}

	outputs, err := p.outputRepo.FindOutputsByTransactionID(ctx, txEntity.ID)
	if err != nil {
		return fmt.Errorf("failed to find inputs and outputs of transaction: %w", err)
	}

	err = p.validateNewTxOutputs(tx, outputs)
	if err != nil {
		return err
	}

	if p.commissionCfg.Satoshis > 0 {
		if err := p.validateCommission(ctx, userID, txEntity.ID, outputs); err != nil {
			return fmt.Errorf("commission validation failed: %w", err)
		}
	}

	// TODO: Add db transactionID to KnownTx.Notify

	// TODO: Remove too long locking scripts (len > storage.maxOutputScript)

	newTxStatus, newReqStatus := p.newStatuses(args)

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

func (p *process) validateStateOfTableTx(reference string, tableTx *entity.Transaction) error {
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

func (p *process) validateNewTxOutputs(tx *transaction.Transaction, outputs []*entity.Output) error {
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

func (p *process) validateCommission(ctx context.Context, userID int, transactionID uint, outputs []*entity.Output) error {
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
		func(output *entity.Output) bool {
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

	return
}

func (p *process) broadcastTxs(ctx context.Context, txIDs []string, isDelayed bool) (*wdk.ProcessActionResult, error) {
	knownTxStatusesLookup, err := p.getKnownTxStatuses(ctx, txIDs...)
	if err != nil {
		return nil, err
	}

	sendWithResults := make([]wdk.SendWithResult, 0, len(txIDs))
	notDelayedResults := make([]wdk.ReviewActionResult, 0, to.IfThen(!isDelayed, len(txIDs)).ElseThen(0))
	var readyToSendTxIDs []string

	for txID, currentStatus := range knownTxStatusesLookup {
		if currentStatus.AlreadySent() {
			sendWithResults = append(sendWithResults, wdk.SendWithResult{
				TxID:   primitives.TXIDHexString(txID),
				Status: currentStatus.SendWithResultStatus(),
			})

			utxoStatus := wdk.UTXOStatusUnproven
			if currentStatus == wdk.ProvenTxStatusCompleted {
				utxoStatus = wdk.UTXOStatusMined
			}

			err = p.outputRepo.MakeOutputsSpendable(ctx, txID, utxoStatus)
			if err != nil {
				return nil, fmt.Errorf("failed to make outputs spendable for txID %s: %w", txID, err)
			}
		} else {
			readyToSendTxIDs = append(readyToSendTxIDs, txID)
		}
	}

	if len(sendWithResults) == len(txIDs) {
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

	beef, err := p.knownTxRepo.GetBEEFForTxIDs(ctx, seq.FromSlice(readyToSendTxIDs), entity.WithStatusesToFilterOut(wdk.ProvenTxReqProblematicStatuses...))
	if err != nil {
		return nil, fmt.Errorf("failed to build valid BEEF: %w", err)
	}

	if ok, err := beef.Verify(ctx, p.services, false); err != nil {
		return nil, fmt.Errorf("failed to verify beef: %w", err)
	} else if !ok {
		return nil, fmt.Errorf("provided beef is not valid")
	}

	// TODO: Create batch string which will be necessary for CRON job to rebuild the BEEF when multiple txs are broadcasted

	if isDelayed {
		resultsForDelayedTxs, err := p.processDelayedTransactions(ctx, readyToSendTxIDs, beef)
		if err != nil {
			return nil, broadcastError.NewBroadcastingError(err, broadcastError.DelayedBroadcast).
				WithPrimaryTxID(readyToSendTxIDs)
		}

		sendWithResults = append(sendWithResults, resultsForDelayedTxs...)

		return &wdk.ProcessActionResult{
			SendWithResults: sendWithResults,
		}, nil
	}

	results, err := p.services.PostBEEF(ctx, beef, readyToSendTxIDs)
	if err != nil {
		return nil, broadcastError.NewBroadcastingError(err, broadcastError.ImmediateBroadcast).
			WithPrimaryTxID(readyToSendTxIDs).
			WithPostBEEFResults(results).
			WithBEEFData(p.logger, beef, nil)
	}

	var (
		sendWithResult     wdk.SendWithResult
		reviewActionResult wdk.ReviewActionResult
	)

	aggregated := results.Aggregated(txIDs)
	for _, broadcastedTxID := range readyToSendTxIDs {
		aggBroadcastResult, ok := aggregated[broadcastedTxID]
		if !ok {
			sendWithResult, reviewActionResult = p.failedResultForTxID(broadcastedTxID)
		} else {
			sendWithResult, reviewActionResult, err = p.updateSingleTx(
				ctx,
				broadcastedTxID,
				aggBroadcastResult,
				results.ServiceErrors(),
				beef,
				readyToSendTxIDs,
			)
			if err != nil {
				processResult := &wdk.ProcessActionResult{
					SendWithResults:   sendWithResults,
					NotDelayedResults: notDelayedResults,
				}
				return nil, broadcastError.NewBroadcastingError(err, broadcastError.ImmediateBroadcast).
					WithContext(processResult, broadcastedTxID, "").
					WithPostBEEFResults(results).
					WithBEEFData(p.logger, beef, nil)
			}
		}

		sendWithResults = append(sendWithResults, sendWithResult)
		notDelayedResults = append(notDelayedResults, reviewActionResult)
	}

	return &wdk.ProcessActionResult{
		SendWithResults:   sendWithResults,
		NotDelayedResults: notDelayedResults,
	}, nil
}

func (p *process) processDelayedTransactions(ctx context.Context, txIDs []string, beef *transaction.Beef) ([]wdk.SendWithResult, error) {
	sendWithResults := make([]wdk.SendWithResult, 0, len(txIDs))
	for _, txID := range txIDs {
		err := p.knownTxRepo.UpdateKnownTxStatus(ctx, txID, wdk.ProvenTxStatusUnsent, wdk.ProvenTxReqBeyondBroadcastStageStatuses, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to update known tx status for txID %s: %w", txID, err)
		}

		err = p.txRepo.UpdateTransactionStatusByTxID(ctx, txID, wdk.TxStatusSending)
		if err != nil {
			return nil, fmt.Errorf("failed to update transaction status for txID %s: %w", txID, err)
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
) (
	sendWithResult wdk.SendWithResult,
	reviewActionResult wdk.ReviewActionResult,
	err error,
) {
	var (
		newReqStatus  wdk.ProvenTxReqStatus
		newTxStatus   wdk.TxStatus
		newUtxoStatus wdk.UTXOStatus
	)

	newReqStatus, newTxStatus, newUtxoStatus, reviewActionResult, sendWithResult, err = p.singleTxBroadcastResult(aggBroadcastResult, txID)
	if err != nil {
		return
	}

	notes := p.notesForPostBEEF(newReqStatus, aggBroadcastResult, serviceErrors, beef, txIDs)

	err = p.txRepo.UpdateTransactionStatusByTxID(ctx, txID, newTxStatus)
	if err != nil {
		err = fmt.Errorf("failed to update transaction status after broadcast: %w", err)
		return
	}

	err = p.knownTxRepo.UpdateKnownTxStatus(ctx, txID, newReqStatus, wdk.ProvenTxReqBeyondBroadcastStageStatuses, notes)
	if err != nil {
		err = fmt.Errorf("failed to update transaction status after broadcast: %w", err)
		return
	}

	if newUtxoStatus != wdk.UTXOStatusUnknown {
		err = p.outputRepo.MakeOutputsSpendable(ctx, txID, newUtxoStatus)
		if err != nil {
			err = fmt.Errorf("failed to make outputs spendable after broadcast: %w", err)
			return
		}
	}

	return
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
		slices.AppendSeq(records, errorNotes)
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

func (p *process) singleTxBroadcastResult(aggBroadcastResult *wdk.AggregatedPostedTxID, txID string) (
	reqStatus wdk.ProvenTxReqStatus,
	txStatus wdk.TxStatus,
	utxoStatus wdk.UTXOStatus,
	reviewActionResult wdk.ReviewActionResult,
	sendWithResult wdk.SendWithResult,
	err error,
) {
	reviewActionResult = wdk.ReviewActionResult{
		TxID: primitives.TXIDHexString(txID),
	}

	sendWithResult = wdk.SendWithResult{
		TxID: primitives.TXIDHexString(txID),
	}

	switch aggBroadcastResult.Status {
	case wdk.AggregatedPostedTxIDSuccess:
		reqStatus = wdk.ProvenTxStatusUnmined
		txStatus = wdk.TxStatusUnproven
		utxoStatus = wdk.UTXOStatusUnproven
		sendWithResult.Status = wdk.SendWithResultStatusUnproven
		reviewActionResult.Status = wdk.ReviewActionResultStatusSuccess
	case wdk.AggregatedPostedTxIDDoubleSpend:
		reqStatus = wdk.ProvenTxStatusDoubleSpend
		txStatus = wdk.TxStatusFailed
		utxoStatus = wdk.UTXOStatusUnknown
		sendWithResult.Status = wdk.SendWithResultStatusFailed
		reviewActionResult.Status = wdk.ReviewActionResultStatusDoubleSpend
		reviewActionResult.CompetingTxs = seq.Collect(maps.Keys(aggBroadcastResult.CompetingTxs))
		// TODO: Build reviewActionResult.CompetingBeef
	case wdk.AggregatedPostedTxIDInvalidTx:
		reqStatus = wdk.ProvenTxStatusInvalid
		txStatus = wdk.TxStatusFailed
		utxoStatus = wdk.UTXOStatusUnknown
		sendWithResult.Status = wdk.SendWithResultStatusFailed
		reviewActionResult.Status = wdk.ReviewActionResultStatusInvalidTx
	case wdk.AggregatedPostedTxIDServiceError:
		// TODO: make sure, this tx will be attempted to be sent again in a periodic task (TaskSendWaiting)
		reqStatus = wdk.ProvenTxStatusSending
		txStatus = wdk.TxStatusSending
		utxoStatus = wdk.UTXOStatusSending
		sendWithResult.Status = wdk.SendWithResultStatusSending
		reviewActionResult.Status = wdk.ReviewActionResultStatusServiceError
	default:
		err = fmt.Errorf("unknown AggregatedPostedTxIDStatus %s", aggBroadcastResult.Status)
	}

	return
}

func (p *process) StopBackgroundBroadcaster() {
	if p.backgroundBroadcaster != nil {
		p.backgroundBroadcaster.Stop()
	}
}

func (p *process) BackgroundBroadcast(ctx context.Context, beef *transaction.Beef, txIDs []string) error {
	results, err := p.services.PostBEEF(ctx, beef, txIDs)
	if err != nil {
		return broadcastError.NewBroadcastingError(err, broadcastError.BackgroundBroadcast).
			WithPostBEEFResults(results).
			WithBEEFData(p.logger, beef, nil)
	}

	aggregated := results.Aggregated(txIDs)
	for _, broadcastedTxID := range txIDs {
		aggBroadcastResult, ok := aggregated[broadcastedTxID]
		if !ok {
			return broadcastError.NewBroadcastingError(
				fmt.Errorf("no broadcast result found for txID %s", broadcastedTxID),
				broadcastError.BackgroundBroadcast,
			).
				WithTxID(broadcastedTxID).
				WithPostBEEFResults(results).
				WithBEEFData(p.logger, beef, nil)
		}

		sendWithResult, _, err := p.updateSingleTx(
			ctx,
			broadcastedTxID,
			aggBroadcastResult,
			results.ServiceErrors(),
			beef,
			txIDs,
		)
		if err != nil {
			return broadcastError.NewBroadcastingError(fmt.Errorf("failed to update single tx after background broadcast: %w", err), broadcastError.BackgroundBroadcast).
				WithTxID(broadcastedTxID).
				WithSendWithResults([]wdk.SendWithResult{sendWithResult}).
				WithPostBEEFResults(results).
				WithBEEFData(p.logger, beef, nil)
		}

		p.logger.DebugContext(ctx, "Background broadcast result", "txID", broadcastedTxID, "status", sendWithResult.Status)
	}

	return nil
}
