package bitails

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-resty/resty/v2"
)

func retryOnTooManyRequestsStatus(res *resty.Response, err error) bool {
	return res.StatusCode() == http.StatusTooManyRequests
}

func extractRawTransactions(beef *transaction.Beef, txIDs []string) ([][]byte, error) {
	rawTxs := make([][]byte, len(txIDs))
	for i, txid := range txIDs {
		tx := beef.FindTransaction(txid)
		if tx == nil {
			return nil, fmt.Errorf("cannot find transaction %s in BEEF", txid)
		}
		raw := tx.Bytes()
		if len(raw) == 0 {
			return nil, fmt.Errorf("empty raw transaction for %s", txid)
		}
		rawTxs[i] = raw
	}
	return rawTxs, nil
}

func convertNotes(notes []string) wdk.Notes {
	converted := make(wdk.Notes, len(notes))
	for i, note := range notes {
		now := time.Now()
		converted[i] = wdk.ReqHistoryNote{
			When: &now,
			What: note,
		}
	}
	return converted
}

func ptrNow() *time.Time {
	now := time.Now()
	return &now
}

func classifyBroadcastStatus(err error) (alreadyKnown, doubleSpend bool, note string) {
	if err == nil {
		return false, false, ""
	}
	switch {
	case errors.Is(err, ErrAlreadyKnown):
		return true, false, "Transaction already in mempool"
	case errors.Is(err, ErrMissingInputs):
		return false, true, "Missing inputs (double spend)"
	default:
		return false, false, err.Error()
	}
}
