package bitails

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
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

func (b *Bitails) broadcast(ctx context.Context, rawTx []byte) wdk.PostedTxID {
	rawHex := hex.EncodeToString(rawTx)
	txid := txutils.TransactionIDFromRawTx(rawTx)

	respArr, err := b.sendBroadcastRequest(ctx, rawHex)
	if err != nil {
		return b.errorPostedTxID(rawTx, txid, fmt.Errorf("broadcast failed for txid %s: %w", txid, err))
	}
	if len(respArr) != 1 {
		return b.errorPostedTxID(rawTx, txid, fmt.Errorf("%s returned %d elements, expected 1", ServiceName, len(respArr)))
	}

	resp := respArr[0]
	result := wdk.PostedTxID{TxID: txid}

	if resp.TxID != "" && resp.TxID != txid {
		return b.errorPostedTxID(rawTx, txid, fmt.Errorf("returned txid (%s) does not match expected txid (%s)", resp.TxID, txid))
	}

	shouldReturnError := b.classifyResponseError(resp, &result)
	if shouldReturnError {
		msg := fmt.Sprintf("broadcasted tx %s with problematic result %s", txid, result.Result)
		if result.Error != nil {
			msg += fmt.Sprintf(" and error: %v", result.Error)
		}
		result.Notes = history.NewBuilder().PostBeefError(ServiceName, history.Hex(rawHex), []string{txid}, msg).Note().AsList()
		return result
	}

	result.Notes = history.NewBuilder().PostBeefSuccess(ServiceName, []string{txid}).Note().AsList()

	info, infoErr := b.fetchTxInfo(ctx, txid)
	if infoErr != nil {
		return b.errorPostedTxID(rawTx, txid, fmt.Errorf("failed to fetch tx info for %s: %w", txid, infoErr))
	}
	if info != nil {
		result.BlockHash = info.BlockHash
		result.BlockHeight = info.BlockHeight
	}

	return result
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
		if r != nil {
			b.logger.DebugContext(ctx, "broadcast request failed with response", "url", url, "error", err, "status_code", r.StatusCode(), "response", r.String())
		}
		return nil, fmt.Errorf("%s request failed: %w", ServiceName, err)
	}
	if r.StatusCode() != HTTPStatusOK && r.StatusCode() != HTTPStatusCreated {
		return nil, fmt.Errorf("%s returned HTTP %d: %s", ServiceName, r.StatusCode(), r.String())
	}

	return respArr, nil
}

func (b *Bitails) classifyResponseError(resp broadcastResponse, result *wdk.PostedTxID) (shouldReturnError bool) {
	if resp.Error == nil {
		result.Result = wdk.PostedTxIDResultSuccess
		return
	}

	msg := resp.Error.Message
	result.Data = fmt.Sprintf("code=%d, msg=%s", resp.Error.Code, msg)

	switch resp.Error.Code {
	case ErrorCodeAlreadyInMempool:
		result.Result = wdk.PostedTxIDResultAlreadyKnown
		result.AlreadyKnown = true
	case ErrorCodeDoubleSpend:
		result.Result = wdk.PostedTxIDResultDoubleSpend
		result.DoubleSpend = true
		shouldReturnError = true
	case ErrorCodeMissingInputs:
		result.Result = wdk.PostedTxIDResultMissingInputs
		shouldReturnError = true
	default:
		result.Result = wdk.PostedTxIDResultError
		result.Error = fmt.Errorf("broadcast error code %d: %s", resp.Error.Code, msg)
		upperMsg := strings.ToUpper(msg)
		if strings.Contains(upperMsg, ErrorTokenECONNRESET) || strings.Contains(upperMsg, ErrorTokenECONNREFUSED) {
			if strings.Contains(upperMsg, ErrorTokenECONNREFUSED) {
				result.Error = fmt.Errorf("broadcast error %s: %s", ErrorTokenECONNREFUSED, msg)
			} else {
				result.Error = fmt.Errorf("broadcast error %s: %s", ErrorTokenECONNRESET, msg)
			}
		} else {
			result.Error = fmt.Errorf("broadcast error code %d: %s", resp.Error.Code, msg)
		}
		shouldReturnError = true
	}

	return
}

func (b *Bitails) errorPostedTxID(raw []byte, txID string, err error) wdk.PostedTxID {
	return wdk.PostedTxID{
		TxID:   txID,
		Result: wdk.PostedTxIDResultError,
		Error:  err,
		Notes:  history.NewBuilder().PostBeefError(ServiceName, history.Bytes(raw), []string{txID}, err.Error()).Note().AsList(),
	}
}
