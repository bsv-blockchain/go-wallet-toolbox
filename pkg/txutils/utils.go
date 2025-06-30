package txutils

import (
	"fmt"
	"net/http"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-resty/resty/v2"
)

// ConvertNotes converts a slice of strings into wdk.Notes, assigning the current time to each note.
func ConvertNotes(notes []string) wdk.Notes {
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

// ExtractRawTransactions extracts raw transaction bytes from a BEEF object based on the provided transaction IDs.
func ExtractRawTransactions(beef *transaction.Beef, txIDs []string) ([][]byte, error) {
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

// RetryOnTooManyRequestsStatus is a retry condition function for resty that checks if the response status code is 429 (Too Many Requests).
func RetryOnTooManyRequestsStatus(res *resty.Response, err error) bool {
	return res.StatusCode() == http.StatusTooManyRequests
}
