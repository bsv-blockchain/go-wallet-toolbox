package whatsonchain

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

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

// broadcast submits a raw transaction to WhatsOnChain.
func (woc *WhatsOnChain) broadcast(ctx context.Context, rawTx []byte) (BroadcastStatus, string, error) {
	rawTxHex := hex.EncodeToString(rawTx)
	txid := txutils.TransactionIDFromRawTx(rawTx)

	type txResponse struct {
		Txid string `json:"txid"`
	}
	var txResp txResponse

	req := woc.httpClient.
		R().
		SetContext(ctx).
		SetResult(&txResp).
		SetBody(broadcastRequest{TxHex: rawTxHex}).
		AddRetryCondition(retryOnTooManyRequestsStatus)

	url := fmt.Sprintf("%s/tx/raw", woc.url)
	res, err := req.Post(url)
	if err != nil {
		return StatusError, "", fmt.Errorf("failed to send request to WoC: %w", err)
	}

	responseText := res.String()

	if res.StatusCode() != http.StatusOK {
		switch {
		case strings.Contains(responseText, "already in mempool"):
			return StatusAlreadyBroadcasted, txid, nil
		case strings.Contains(responseText, "txn-mempool-conflict"):
			return StatusDoubleSpend, txid, nil
		case strings.Contains(responseText, "missing inputs"):
			return StatusMissingInputs, txid, nil
		default:
			return StatusError, "", fmt.Errorf("woc returned error %d: %s", res.StatusCode(), responseText)
		}
	}

	if txResp.Txid != txid {
		return StatusError, txResp.Txid, fmt.Errorf("woc returned txid %s does not match local calculated txid %s", txResp.Txid, txid)
	}

	return StatusSuccess, txid, nil
}

func convertNotes(notes []string) wdk.Notes {
	converted := make(wdk.Notes, len(notes))
	for i, note := range notes {
		now := time.Now()
		converted[i] = wdk.ReqHistoryNote{
			When: &now,
			What: note,
			Args: nil,
		}
	}
	return converted
}
