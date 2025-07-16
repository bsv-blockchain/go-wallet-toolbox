package bitails

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type broadcastRequest struct {
	Raws []string `json:"raws"`
}
type broadcastResponse struct {
	TxID  string          `json:"txid"`
	Error *broadcastError `json:"error,omitempty"`
}
type broadcastError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (b *Bitails) broadcast(ctx context.Context, rawTx []byte) (*wdk.PostedTxID, error) {
	rawHex := hex.EncodeToString(rawTx)
	txid := txutils.TransactionIDFromRawTx(rawTx)

	respArr, err := b.sendBroadcastRequest(ctx, rawHex)
	if err != nil {
		return nil, fmt.Errorf("broadcast failed for txid %s: %w", txid, err)
	}
	if len(respArr) != 1 {
		return nil, fmt.Errorf("%s returned %d elements, expected 1", ServiceName, len(respArr))
	}

	resp := respArr[0]
	result := &wdk.PostedTxID{TxID: txid}

	if resp.TxID != "" && resp.TxID != txid {
		return nil, fmt.Errorf("returned txid (%s) does not match expected txid (%s)", resp.TxID, txid)
	}

	broadcastErr := b.classifyResponseError(resp, result)
	if broadcastErr != nil {
		// NOTE: Result is returned along with the error to provide already calculated data
		return result, fmt.Errorf("broadcast error for txid %s: %w", txid, err)
	}

	info, infoErr := b.fetchTxInfo(ctx, txid)
	if infoErr != nil {
		return nil, fmt.Errorf("failed to fetch tx info for %s: %w", txid, err)
	}
	if info != nil {
		result.BlockHash = info.BlockHash
		result.BlockHeight = info.BlockHeight
	}

	return result, nil
}

func (b *Bitails) sendBroadcastRequest(ctx context.Context, rawHex string) ([]broadcastResponse, error) {
	reqBody := broadcastRequest{Raws: []string{rawHex}}
	var respArr []broadcastResponse

	url, err := broadcastURL(b.url)
	if err != nil {
		return nil, fmt.Errorf("failed to build broadcast URL: %w", err)
	}

	r, err := b.httpClient.R().
		SetContext(ctx).
		SetBody(reqBody).
		SetResult(&respArr).
		Post(url)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %w", ServiceName, err)
	}
	if r.StatusCode() != HTTPStatusOK && r.StatusCode() != HTTPStatusCreated {
		return nil, fmt.Errorf("%s returned HTTP %d: %s", ServiceName, r.StatusCode(), r.String())
	}

	return respArr, nil
}

func (b *Bitails) classifyResponseError(resp broadcastResponse, result *wdk.PostedTxID) error {
	if resp.Error == nil {
		result.Result = wdk.PostedTxIDResultSuccess
		return nil
	}

	msg := resp.Error.Message
	result.Data = fmt.Sprintf("code=%d, msg=%s", resp.Error.Code, msg)

	switch resp.Error.Code {
	case ErrorCodeAlreadyInMempool:
		result.Result = wdk.PostedTxIDResultAlreadyKnown
		result.AlreadyKnown = true
		return nil
	case ErrorCodeMissingInputs:
		result.Result = wdk.PostedTxIDResultDoubleSpend
		result.DoubleSpend = true
		return ErrMissingInputs
	default:
		result.Result = wdk.PostedTxIDResultError
		return fmt.Errorf("broadcast error code %d: %s", resp.Error.Code, msg)
	}
}
