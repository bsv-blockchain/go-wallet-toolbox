package whatsonchain

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/go-softwarelab/common/pkg/to"
	wocsdk "github.com/mrz1836/go-whatsonchain"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// PostTX broadcasts a single raw transaction to WhatsOnChain. It never returns a
// Go error: broadcast outcomes (success, already-known, double-spend, missing
// inputs, error) are folded into the returned wdk.PostedTxID so the broadcast
// router can decide how to proceed.
func (woc *WhatsOnChain) PostTX(ctx context.Context, rawTx []byte) (_ *wdk.PostedTxID, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-PostTX", attribute.String("service", "whatsonchain"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	result := woc.processSingleTx(ctx, rawTx)
	return &result, nil
}

func (woc *WhatsOnChain) processSingleTx(ctx context.Context, rawTx []byte) wdk.PostedTxID {
	txid := txutils.TransactionIDFromRawTx(rawTx)

	if err := woc.wait(ctx); err != nil {
		return woc.errorPostedTxID(rawTx, txid, fmt.Errorf("broadcast failed for txid %s: %w", txid, err))
	}

	_, err := woc.client.BroadcastTx(ctx, hex.EncodeToString(rawTx))

	result := wdk.PostedTxID{TxID: txid}
	if err != nil {
		if shouldReturnError := classifyBroadcastError(err, &result); shouldReturnError {
			msg := fmt.Sprintf("broadcasted tx %s with problematic result %s", txid, result.Result)
			if result.Error != nil {
				msg += fmt.Sprintf(" and error: %v", result.Error)
			}
			result.Notes = history.NewBuilder().PostBeefError(ServiceName, history.Bytes(rawTx), []string{txid}, msg).Note().AsList()
			return result
		}
		// Non-error classification (already in mempool): treat as success.
	} else {
		result.Result = wdk.PostedTxIDResultSuccess
	}

	result.Notes = history.NewBuilder().PostBeefSuccess(ServiceName, []string{txid}).Note().AsList()
	woc.enrichWithBlockInfo(ctx, &result, txid)
	return result
}

// classifyBroadcastError maps an SDK broadcast error onto the PostedTxID result and
// reports whether the outcome is a failure the router should treat as an error.
// An "already in mempool" outcome is not an error - the transaction is accepted.
func classifyBroadcastError(err error, result *wdk.PostedTxID) (shouldReturnError bool) {
	switch {
	case errors.Is(err, wocsdk.ErrTxAlreadyInMempool):
		result.Result = wdk.PostedTxIDResultAlreadyKnown
		result.AlreadyKnown = true
		return false
	case errors.Is(err, wocsdk.ErrTxMempoolConflict):
		result.Result = wdk.PostedTxIDResultDoubleSpend
		result.DoubleSpend = true
		return true
	case errors.Is(err, wocsdk.ErrTxMissingInputs):
		result.Result = wdk.PostedTxIDResultMissingInputs
		result.DoubleSpend = true
		return true
	default:
		result.Result = wdk.PostedTxIDResultError
		result.Error = err
		return true
	}
}

// enrichWithBlockInfo best-effort populates block hash/height for an accepted tx.
// A failure here must not void the successful broadcast: right after a broadcast
// the tx may not be indexed yet (or the endpoint may be rate limited).
func (woc *WhatsOnChain) enrichWithBlockInfo(ctx context.Context, result *wdk.PostedTxID, txid string) {
	if err := woc.wait(ctx); err != nil {
		woc.logger.WarnContext(ctx, "failed to fetch tx info after successful broadcast", "txid", txid, "error", err)
		return
	}

	statuses, err := woc.client.BulkTransactionStatus(ctx, &wocsdk.TxHashes{TxIDs: []string{txid}})
	if err != nil || len(statuses) == 0 || statuses[0] == nil {
		woc.logger.WarnContext(ctx, "failed to fetch tx info after successful broadcast", "txid", txid, "error", err)
		return
	}

	status := statuses[0]
	result.BlockHash = status.BlockHash
	if status.BlockHeight > 0 {
		if height, convErr := to.UInt32(status.BlockHeight); convErr == nil {
			result.BlockHeight = height
		}
	}
}

func (woc *WhatsOnChain) errorPostedTxID(raw []byte, txID string, err error) wdk.PostedTxID {
	return wdk.PostedTxID{
		TxID:   txID,
		Result: wdk.PostedTxIDResultError,
		Error:  err,
		Notes:  history.NewBuilder().PostBeefError(ServiceName, history.Bytes(raw), []string{txID}, err.Error()).Note().AsList(),
	}
}
