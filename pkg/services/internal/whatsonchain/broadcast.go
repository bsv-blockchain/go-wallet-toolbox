package whatsonchain

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/utils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
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

type txBroadcastResponse struct {
	TxID string `json:"txid"`
}

type txInfoResult struct {
	BlockHash   string
	BlockHeight int64
}

func (woc *WhatsOnChain) broadcast(ctx context.Context, rawTx []byte) (BroadcastStatus, string, error) {
	rawTxHex := hex.EncodeToString(rawTx)
	txid := txutils.TransactionIDFromRawTx(rawTx)

	url := fmt.Sprintf("%s/tx/raw", woc.url)

	var resp txBroadcastResponse

	req := woc.httpClient.
		R().
		SetContext(ctx).
		SetResult(&resp).
		SetBody(broadcastRequest{TxHex: rawTxHex}).
		AddRetryCondition(retryOnTooManyRequestsStatus)

	res, err := req.Post(url)
	if err != nil {
		return StatusError, "", fmt.Errorf("failed to send request to WoC: %w", err)
	}

	if res.StatusCode() != http.StatusOK {
		responseText := res.String()

		switch {
		case containsI(responseText, "already in mempool", "txn-already-known"):
			return StatusAlreadyBroadcasted, txid, nil
		case containsI(responseText, "txn-mempool-conflict"):
			return StatusDoubleSpend, txid, nil
		case containsI(responseText, "missing inputs"):
			return StatusMissingInputs, txid, nil
		default:
			return StatusError, "", fmt.Errorf("woc returned unexpected error %d: %s", res.StatusCode(), responseText)
		}
	}

	if resp.TxID != txid {
		return StatusError, "", fmt.Errorf("txid mismatch: expected %s, got %s", txid, resp.TxID)
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
		BlockHeight   int64  `json:"blockheight"`
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
	status, returnedTxid, broadcastErr := woc.broadcast(ctx, rawTx)

	if broadcastErr != nil {
		return wdk.PostedTxID{
			Result: wdk.PostedTxIDResultError,
			TxID:   txid,
			Error:  broadcastErr,
			Notes:  utils.ConvertNotes([]string{fmt.Sprintf("broadcast error: %v", broadcastErr)}),
		}
	}

	resultStatus, notes := classifyBroadcastStatus(status)

	txInfo, fetchErr := woc.tryFetchTxInfo(ctx, returnedTxid)
	if fetchErr != nil {
		notes = append(notes, fmt.Sprintf("failed to fetch tx info: %v", fetchErr))
	}

	var blockHash string
	var blockHeight int64
	if txInfo != nil {
		blockHash = txInfo.BlockHash
		blockHeight = txInfo.BlockHeight
	}

	return wdk.PostedTxID{
		Result:       resultStatus,
		TxID:         returnedTxid,
		DoubleSpend:  status == StatusDoubleSpend || status == StatusMissingInputs,
		AlreadyKnown: status == StatusAlreadyBroadcasted,
		BlockHash:    blockHash,
		BlockHeight:  blockHeight,
		Error:        firstNonNilError(fetchErr),
		Notes:        utils.ConvertNotes(notes),
		// NOTE: MerklePath is not fetched here because that would require additional API call
	}
}

func (woc *WhatsOnChain) tryFetchTxInfo(ctx context.Context, txid string) (*txInfoResult, error) {
	return woc.fetchTxInfo(ctx, txid)
}
