package bitails

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
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
		msg := fmt.Sprintf("%s returned %d elements, expected 1", ServiceName, len(respArr))
		return &wdk.PostedTxID{
			TxID:   txid,
			Result: wdk.PostedTxIDResultError,
			Error:  fmt.Errorf("%s", msg),
			Notes:  convertNotes([]string{msg}),
		}, nil
	}

	resp := respArr[0]
	result := &wdk.PostedTxID{TxID: txid}

	maybeErr := b.classifyResponseError(resp, result)

	if resp.TxID != "" && resp.TxID != txid {
		result.Notes = append(result.Notes, wdk.ReqHistoryNote{
			When: ptrNow(),
			What: "Returned TxID mismatch",
		})
		result.Result = wdk.PostedTxIDResultError
		return result, fmt.Errorf("returned txid (%s) does not match expected txid (%s)", resp.TxID, txid)
	}

	info, fetchErr := b.fetchTxInfo(ctx, txid)
	if fetchErr != nil && maybeErr == nil {
		maybeErr = fmt.Errorf("error fetching tx info: %w", fetchErr)
	}
	if info != nil {
		result.BlockHash = info.BlockHash
		result.BlockHeight = info.BlockHeight
	}

	already, double, note := classifyBroadcastStatus(maybeErr)
	result.AlreadyKnown = result.AlreadyKnown || already
	result.DoubleSpend = result.DoubleSpend || double
	if note != "" {
		result.Notes = append(result.Notes, wdk.ReqHistoryNote{When: ptrNow(), What: note})
	}
	if maybeErr != nil && !(already || double) {
		result.Error = maybeErr
	}

	return result, nil
}

func (b *Bitails) sendBroadcastRequest(ctx context.Context, rawHex string) ([]broadcastResponse, error) {
	reqBody := broadcastRequest{Raws: []string{rawHex}}
	var respArr []broadcastResponse

	r, err := b.httpClient.R().
		SetContext(ctx).
		SetBody(reqBody).
		SetResult(&respArr).
		Post(b.url + BroadcastEndpoint)
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
	result.Notes = convertNotes([]string{msg})

	switch resp.Error.Code {
	case ErrorCodeAlreadyInMempool:
		result.Result = wdk.PostedTxIDResultAlreadyKnown
		result.AlreadyKnown = true
		return ErrAlreadyKnown
	case ErrorCodeMissingInputs:
		result.Result = wdk.PostedTxIDResultDoubleSpend
		result.DoubleSpend = true
		return ErrMissingInputs
	default:
		result.Result = wdk.PostedTxIDResultError
		return fmt.Errorf("broadcast error code %d: %s", resp.Error.Code, msg)
	}
}
