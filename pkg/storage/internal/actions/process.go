package actions

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"maps"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/seq"
)

type process struct {
	logger      *slog.Logger
	txRepo      TransactionsRepo
	outputRepo  OutputRepo
	knownTxRepo KnownTxRepo
	services    wdk.Services
}

func newProcessAction(logger *slog.Logger, txRepo TransactionsRepo, outputRepo OutputRepo, knownTxRepo KnownTxRepo, services wdk.Services) *process {
	logger = logging.Child(logger, "processAction")
	return &process{
		logger:      logger,
		txRepo:      txRepo,
		outputRepo:  outputRepo,
		knownTxRepo: knownTxRepo,
		services:    services,
	}
}

func (p *process) Process(ctx context.Context, userID int, args *wdk.ProcessActionArgs) (*wdk.ProcessActionResult, error) {
	if args.IsNewTx {
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

	return p.broadcastSingleTx(ctx, string(*args.TxID))
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

	// TODO: Commission; but it requires Commission table (it needs to be created & new rows added during "createAction"

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
	}, history.ProcessActionHistoryNote, history.UserIDHistoryAttr(userID))
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

func (p *process) broadcastSingleTx(ctx context.Context, txID string) (*wdk.ProcessActionResult, error) {
	sendStatus, err := p.getSendStatus(ctx, txID)
	if err != nil {
		return nil, err
	}

	if sendStatus != wdk.SendWithResultStatusSending {
		return &wdk.ProcessActionResult{
			SendWithResults: []wdk.SendWithResult{
				{
					TxID:   primitives.TXIDHexString(txID),
					Status: sendStatus,
				},
			},
		}, nil
	}

	beef, err := p.knownTxRepo.BuildValidBEEF(ctx, txID, wdk.ProvenTxReqProblematicStatuses)
	if err != nil {
		return nil, fmt.Errorf("failed to build valid BEEF: %w", err)
	}

	// TODO: SPV of the beef

	results, err := p.services.PostBEEF(ctx, beef, []string{txID})
	if err != nil {
		return nil, fmt.Errorf("failed to post BEEF: %w", err)
	}

	// TODO: Store notes from PostBEEF result

	aggregated := results.Aggregated([]string{txID})
	aggBroadcastResult, ok := aggregated[txID]
	if !ok {
		return nil, fmt.Errorf("failed to find aggregated result for txID %s", txID)
	}

	newReqStatus, newTxStatus, result, err := p.processBroadcastSingleTxResult(aggBroadcastResult, txID)
	if err != nil {
		return nil, err
	}

	err = p.txRepo.UpdateTransactionStatusForTxID(ctx, txID, newTxStatus, newReqStatus, history.AggregateResultsHistoryNote, p.noteForAggregation(aggBroadcastResult))
	if err != nil {
		return nil, fmt.Errorf("failed to update transaction status after broadcast: %w", err)
	}

	return &result, nil
}

func (p *process) noteForAggregation(aggBroadcastResult *wdk.AggregatedPostedTxID) map[string]any {
	return map[string]any{
		"aggStatus":         aggBroadcastResult.Status,
		"successCount":      aggBroadcastResult.SuccessCount,
		"doubleSpendCount":  aggBroadcastResult.DoubleSpendCount,
		"statusErrorCount":  aggBroadcastResult.StatusErrorCount,
		"serviceErrorCount": aggBroadcastResult.ServiceErrorCount,
	}
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
