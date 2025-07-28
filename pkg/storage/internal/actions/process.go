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
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seq2"
)

type process struct {
	logger         *slog.Logger
	commissionCfg  defs.Commission
	txRepo         TransactionsRepo
	outputRepo     OutputRepo
	knownTxRepo    KnownTxRepo
	commissionRepo CommissionRepo
	services       wdk.Services
}

func newProcessAction(logger *slog.Logger, txRepo TransactionsRepo, commissionCfg defs.Commission, outputRepo OutputRepo, knownTxRepo KnownTxRepo, commissionRepo CommissionRepo, services wdk.Services) *process {
	logger = logging.Child(logger, "processAction")
	return &process{
		logger:         logger,
		commissionCfg:  commissionCfg,
		txRepo:         txRepo,
		outputRepo:     outputRepo,
		knownTxRepo:    knownTxRepo,
		commissionRepo: commissionRepo,
		services:       services,
	}
}

func (p *process) Process(ctx context.Context, userID int, args *wdk.ProcessActionArgs) (*wdk.ProcessActionResult, error) {
	var reference string
	if args.Reference != nil {
		reference = *args.Reference
	}

	p.logger.DebugContext(ctx, "Starting process flow decision",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.Bool("isNewTx", args.IsNewTx),
		slog.Bool("isSendWith", args.IsSendWith),
		slog.Bool("isDelayed", args.IsDelayed),
	)

	if args.IsNewTx {
		p.logger.DebugContext(ctx, "Processing new transaction",
			logging.UserID(userID),
			logging.Reference(reference),
		)
		err := p.processNewTx(ctx, userID, args)
		if err != nil {
			return nil, err
		}
	}

	if args.IsSendWith {
		panic("not implemented yet")
	}

	if args.IsDelayed {
		panic("not implemented yet")
	}

	p.logger.DebugContext(ctx, "Broadcasting single transaction",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.String("txID", string(*args.TxID)),
	)

	return p.broadcastSingleTx(ctx, userID, reference, string(*args.TxID))
}

func (p *process) processNewTx(ctx context.Context, userID int, args *wdk.ProcessActionArgs) error {
	var reference string
	if args.Reference != nil {
		reference = *args.Reference
	}

	p.logger.DebugContext(ctx, "Parsing transaction from raw bytes",
		logging.UserID(userID),
		logging.Reference(reference),
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

	p.logger.DebugContext(ctx, "Transaction parsed successfully",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.String("txID", txID),
		slog.Int("inputCount", len(tx.Inputs)),
		slog.Int("outputCount", len(tx.Outputs)),
	)

	// TODO: Services::nLockTimeIsFinal(tx)

	p.logger.DebugContext(ctx, "Finding transaction by reference",
		logging.UserID(userID),
		logging.Reference(reference),
	)

	txEntity, err := p.txRepo.FindTransactionByReference(ctx, userID, *args.Reference)
	if err != nil {
		return fmt.Errorf("failed to find transaction by reference: %w", err)
	}

	p.logger.DebugContext(ctx, "Validating transaction state",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.String("txStatus", string(txEntity.Status)),
		slog.Bool("isOutgoing", txEntity.IsOutgoing),
	)

	err = p.validateStateOfTableTx(*args.Reference, txEntity)
	if err != nil {
		return err
	}

	p.logger.DebugContext(ctx, "Finding outputs for transaction",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.Uint64("transactionID", uint64(txEntity.ID)),
	)

	outputs, err := p.outputRepo.FindOutputsByTransactionID(ctx, txEntity.ID)
	if err != nil {
		return fmt.Errorf("failed to find inputs and outputs of transaction: %w", err)
	}

	p.logger.DebugContext(ctx, "Validating transaction outputs",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.Int("outputCount", len(outputs)),
	)

	err = p.validateNewTxOutputs(tx, outputs)
	if err != nil {
		return err
	}

	if p.commissionCfg.Satoshis > 0 {
		p.logger.DebugContext(ctx, "Validating commission",
			logging.UserID(userID),
			logging.Reference(reference),
			slog.Uint64("commissionSatoshis", p.commissionCfg.Satoshis),
		)

		if err := p.validateCommission(ctx, userID, txEntity.ID, outputs); err != nil {
			return fmt.Errorf("commission validation failed: %w", err)
		}
	} else {
		p.logger.DebugContext(ctx, "Skipping commission validation (not configured)",
			logging.UserID(userID),
			logging.Reference(reference),
		)
	}

	// TODO: Add db transactionID to KnownTx.Notify

	// TODO: Remove too long locking scripts (len > storage.maxOutputScript)

	newTxStatus, newReqStatus := p.newStatuses(ctx, userID, args)

	p.logger.InfoContext(ctx, "Updating transaction with new status",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.String("newTxStatus", string(newTxStatus)),
		slog.String("newReqStatus", string(newReqStatus)),
		slog.String("txID", txID),
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

	p.logger.InfoContext(ctx, "New transaction processed successfully",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.String("txID", txID),
	)

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

func (p *process) newStatuses(ctx context.Context, userID int, args *wdk.ProcessActionArgs) (txStatus wdk.TxStatus, reqStatus wdk.ProvenTxReqStatus) {
	var reference string
	if args.Reference != nil {
		reference = *args.Reference
	}

	switch {
	case args.IsNoSend:
		reqStatus = wdk.ProvenTxStatusNoSend
		txStatus = wdk.TxStatusNoSend
		p.logger.DebugContext(ctx, "Determined NoSend status",
			logging.UserID(userID),
			logging.Reference(reference),
			slog.String("txStatus", string(txStatus)),
			slog.String("reqStatus", string(reqStatus)),
		)
	case args.IsDelayed:
		reqStatus = wdk.ProvenTxStatusUnsent
		txStatus = wdk.TxStatusUnprocessed
		p.logger.DebugContext(ctx, "Determined Delayed status",
			logging.UserID(userID),
			logging.Reference(reference),
			slog.String("txStatus", string(txStatus)),
			slog.String("reqStatus", string(reqStatus)),
		)
	default:
		reqStatus = wdk.ProvenTxStatusUnprocessed
		txStatus = wdk.TxStatusUnprocessed
		p.logger.DebugContext(ctx, "Determined default processing status",
			logging.UserID(userID),
			logging.Reference(reference),
			slog.String("txStatus", string(txStatus)),
			slog.String("reqStatus", string(reqStatus)),
		)
	}

	return
}

func (p *process) broadcastSingleTx(ctx context.Context, userID int, reference, txID string) (*wdk.ProcessActionResult, error) {
	p.logger.DebugContext(ctx, "Getting send status for transaction",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.String("txID", txID),
	)

	sendStatus, err := p.getSendStatus(ctx, txID)
	if err != nil {
		return nil, err
	}

	p.logger.DebugContext(ctx, "Send status determined",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.String("txID", txID),
		slog.String("sendStatus", string(sendStatus)),
	)

	if sendStatus != wdk.SendWithResultStatusSending {
		p.logger.DebugContext(ctx, "Transaction not ready for sending, returning status",
			logging.UserID(userID),
			logging.Reference(reference),
			slog.String("txID", txID),
			slog.String("sendStatus", string(sendStatus)),
		)
		return &wdk.ProcessActionResult{
			SendWithResults: []wdk.SendWithResult{
				{
					TxID:   primitives.TXIDHexString(txID),
					Status: sendStatus,
				},
			},
		}, nil
	}

	p.logger.DebugContext(ctx, "Building BEEF for transaction",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.String("txID", txID),
	)

	beef, err := p.knownTxRepo.BuildValidBEEF(ctx, txID, wdk.ProvenTxReqProblematicStatuses)
	if err != nil {
		return nil, fmt.Errorf("failed to build valid BEEF: %w", err)
	}

	p.logger.DebugContext(ctx, "Verifying BEEF",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.String("txID", txID),
	)

	if ok, err := beef.Verify(ctx, p.services, false); err != nil {
		return nil, fmt.Errorf("failed to verify beef: %w", err)
	} else if !ok {
		return nil, fmt.Errorf("provided beef is not valid")
	}

	p.logger.InfoContext(ctx, "Broadcasting transaction via PostBEEF",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.String("txID", txID),
	)

	results, err := p.services.PostBEEF(ctx, beef, []string{txID})
	if err != nil {
		return nil, fmt.Errorf("failed to post BEEF: %w", err)
	}

	aggregated := results.Aggregated([]string{txID})
	aggBroadcastResult, ok := aggregated[txID]
	if !ok {
		return nil, fmt.Errorf("failed to find aggregated result for txID %s", txID)
	}

	p.logger.DebugContext(ctx, "Processing broadcast result",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.String("txID", txID),
		slog.String("aggStatus", string(aggBroadcastResult.Status)),
		slog.Int("successCount", aggBroadcastResult.SuccessCount),
		slog.Int("doubleSpendCount", aggBroadcastResult.DoubleSpendCount),
		slog.Int("statusErrorCount", aggBroadcastResult.StatusErrorCount),
		slog.Int("serviceErrorCount", aggBroadcastResult.ServiceErrorCount),
	)

	newReqStatus, newTxStatus, result, err := p.processBroadcastSingleTxResult(aggBroadcastResult, txID)
	if err != nil {
		return nil, err
	}

	notes := p.notesForPostBEEF(newReqStatus, aggBroadcastResult, results.ServiceErrors(), beef, []string{txID})

	p.logger.InfoContext(ctx, "Updating transaction status after broadcast",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.String("txID", txID),
		slog.String("newTxStatus", string(newTxStatus)),
		slog.String("newReqStatus", string(newReqStatus)),
		slog.Int("notesCount", len(notes)),
	)

	err = p.txRepo.UpdateTransactionStatusForTxID(ctx, txID, newTxStatus, newReqStatus, notes)
	if err != nil {
		return nil, fmt.Errorf("failed to update transaction status after broadcast: %w", err)
	}

	p.logger.InfoContext(ctx, "Transaction broadcast completed successfully",
		logging.UserID(userID),
		logging.Reference(reference),
		slog.String("txID", txID),
		slog.String("finalStatus", string(newTxStatus)),
	)

	return &result, nil
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

func (p *process) getSendStatus(ctx context.Context, txID string) (wdk.SendWithResultStatus, error) {
	reqTxStatus, err := p.knownTxRepo.FindKnownTxStatus(ctx, txID)
	if err != nil {
		return "", fmt.Errorf("failed to find known tx status: %w", err)
	}

	switch reqTxStatus.BroadcastStatus() {
	case wdk.TxReqBroadcastReadyToSend:
		return wdk.SendWithResultStatusSending, nil
	case wdk.TxReqBroadcastError:
		return wdk.SendWithResultStatusFailed, nil
	case wdk.TxReqBroadcastAlreadySent:
		return wdk.SendWithResultStatusUnproven, nil
	case wdk.TxReqBroadcastUnknown:
		fallthrough
	default:
		return "", fmt.Errorf("unknown broadcast status")
	}
}

func (p *process) processBroadcastSingleTxResult(aggBroadcastResult *wdk.AggregatedPostedTxID, txID string) (
	reqStatus wdk.ProvenTxReqStatus,
	txStatus wdk.TxStatus,
	result wdk.ProcessActionResult,
	err error,
) {
	reviewActionResult := wdk.ReviewActionResult{
		TxID: primitives.TXIDHexString(txID),
	}

	sendWithResult := wdk.SendWithResult{
		TxID: primitives.TXIDHexString(txID),
	}

	switch aggBroadcastResult.Status {
	case wdk.AggregatedPostedTxIDSuccess:
		reqStatus = wdk.ProvenTxStatusUnmined
		txStatus = wdk.TxStatusUnproven
		sendWithResult.Status = wdk.SendWithResultStatusUnproven
		reviewActionResult.Status = wdk.ReviewActionResultStatusSuccess
	case wdk.AggregatedPostedTxIDDoubleSpend:
		reqStatus = wdk.ProvenTxStatusDoubleSpend
		txStatus = wdk.TxStatusFailed
		sendWithResult.Status = wdk.SendWithResultStatusFailed
		reviewActionResult.Status = wdk.ReviewActionResultStatusDoubleSpend
		reviewActionResult.CompetingTxs = seq.Collect(maps.Keys(aggBroadcastResult.CompetingTxs))
		// TODO: Build reviewActionResult.CompetingBeef
	case wdk.AggregatedPostedTxIDInvalidTx:
		reqStatus = wdk.ProvenTxStatusInvalid
		txStatus = wdk.TxStatusFailed
		sendWithResult.Status = wdk.SendWithResultStatusFailed
		reviewActionResult.Status = wdk.ReviewActionResultStatusInvalidTx
	case wdk.AggregatedPostedTxIDServiceError:
		// TODO: make sure, this tx will be attempted to be sent again in a periodic task (TaskSendWaiting)
		reqStatus = wdk.ProvenTxStatusSending
		txStatus = wdk.TxStatusSending
		sendWithResult.Status = wdk.SendWithResultStatusSending
		reviewActionResult.Status = wdk.ReviewActionResultStatusServiceError
	default:
		err = fmt.Errorf("unknown AggregatedPostedTxIDStatus %s", aggBroadcastResult.Status)
	}

	result.SendWithResults = []wdk.SendWithResult{sendWithResult}
	result.NotDelayedResults = []wdk.ReviewActionResult{reviewActionResult}

	return
}
