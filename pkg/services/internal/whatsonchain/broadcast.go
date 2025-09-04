package whatsonchain

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// BroadcastStatus represents the result of broadcasting a transaction
type BroadcastStatus int

const (
	StatusError BroadcastStatus = iota
	StatusSuccess
	StatusAlreadyBroadcasted
	StatusDoubleSpend
	StatusMissingInputs
)

type broadcastRequest struct {
	TxHex string `json:"txhex"`
}

type txInfoResult struct {
	BlockHash   string
	BlockHeight uint32
}

func (woc *WhatsOnChain) broadcast(ctx context.Context, rawTx []byte) (BroadcastStatus, string, error) {
	rawTxHex := hex.EncodeToString(rawTx)
	txid := txutils.TransactionIDFromRawTx(rawTx)

	url := fmt.Sprintf("%s/tx/raw", woc.url)

	req := woc.httpClient.
		R().
		SetContext(ctx).
		SetBody(broadcastRequest{TxHex: rawTxHex})

	res, err := req.Post(url)
	if err != nil {
		if res != nil {
			woc.logger.DebugContext(ctx, "broadcast request failed with response", "url", url, "error", err, "status_code", res.StatusCode(), "response", res.String())
		}
		return StatusError, "", fmt.Errorf("failed to send request to WoC: %w", err)
	}

	if res.StatusCode() != http.StatusOK {
		responseText := res.String()

		switch {
		case containsI(responseText, "already in mempool", "already in the mempool", "txn-already-known"):
			return StatusAlreadyBroadcasted, txid, nil
		case containsI(responseText, "txn-mempool-conflict"):
			return StatusDoubleSpend, txid, nil
		case containsI(responseText, "missing inputs"):
			return StatusMissingInputs, txid, nil
		default:
			return StatusError, "", fmt.Errorf("woc returned unexpected error %d: %s", res.StatusCode(), responseText)
		}
	}

	return StatusSuccess, txid, nil
}

func (woc *WhatsOnChain) fetchTxInfo(ctx context.Context, txid string) (*txInfoResult, error) {
	type wocStatusRequest struct {
		Txids []string `json:"txids"`
	}

	type wocStatusResponse []struct {
		TxID          string `json:"txid"`
		BlockHash     string `json:"blockhash"`
		BlockHeight   uint32 `json:"blockheight"`
		BlockTime     int64  `json:"blocktime"`
		Confirmations int    `json:"confirmations"`
	}

	var resp wocStatusResponse

	url := fmt.Sprintf("%s/txs/status", woc.url)

	req := woc.httpClient.
		R().
		SetContext(ctx).
		SetBody(wocStatusRequest{
			Txids: []string{txid},
		}).
		SetResult(&resp)

	res, err := req.Post(url)
	if err != nil {
		return nil, fmt.Errorf("failed to call WoC: %w", err)
	}

	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected WoC status: %d", res.StatusCode())
	}

	if len(resp) == 0 {
		return nil, fmt.Errorf("no data returned for txid: %s", txid)
	}

	return &txInfoResult{
		BlockHash:   resp[0].BlockHash,
		BlockHeight: resp[0].BlockHeight,
	}, nil
}

func (woc *WhatsOnChain) processSingleTx(ctx context.Context, txid string, rawTx []byte) wdk.PostedTxID {
	status, returnedTxid, err := woc.broadcast(ctx, rawTx)
	if err != nil {
		return woc.errorPostedTxID(rawTx, txid, fmt.Errorf("broadcast failed for txid %s: %w", txid, err))
	}

	result := wdk.PostedTxID{
		TxID: returnedTxid,
	}

	shouldReturnError := classifyBroadcastStatus(status, &result)
	if shouldReturnError {
		msg := fmt.Sprintf("broadcasted tx %s with problematic result %s", txid, result.Result)
		if result.Error != nil {
			msg += fmt.Sprintf(" and error: %v", result.Error)
		}
		result.Notes = history.NewBuilder().PostBeefError(ServiceName, history.Bytes(rawTx), []string{txid}, msg).Note().AsList()
		return result
	}

	result.Notes = history.NewBuilder().PostBeefSuccess(ServiceName, []string{txid}).Note().AsList()

	info, fetchErr := woc.tryFetchTxInfo(ctx, returnedTxid)
	if fetchErr != nil {
		return woc.errorPostedTxID(rawTx, returnedTxid, fmt.Errorf("failed to fetch tx info for %s: %w", returnedTxid, fetchErr))
	}

	if info != nil {
		result.BlockHash = info.BlockHash
		result.BlockHeight = info.BlockHeight
	}

	return result
}

func (woc *WhatsOnChain) tryFetchTxInfo(ctx context.Context, txid string) (*txInfoResult, error) {
	return woc.fetchTxInfo(ctx, txid)
}

func (woc *WhatsOnChain) errorPostedTxID(raw []byte, txID string, err error) wdk.PostedTxID {
	return wdk.PostedTxID{
		TxID:   txID,
		Result: wdk.PostedTxIDResultError,
		Error:  err,
		Notes:  history.NewBuilder().PostBeefError(ServiceName, history.Bytes(raw), []string{txID}, err.Error()).Note().AsList(),
	}
}
