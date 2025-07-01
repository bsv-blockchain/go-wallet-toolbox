package txutils

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
)

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
