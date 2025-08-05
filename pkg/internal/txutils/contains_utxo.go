package txutils

import (
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// ContainsUtxo checks if the provided outpoint exists within the UTXO details slice.
func ContainsUtxo(details []wdk.UtxoDetail, outpoint *transaction.Outpoint) bool {
	outpointTxID := outpoint.Txid.String()
	for _, d := range details {
		if d.TxID == outpointTxID && d.Index == outpoint.Index {
			return true
		}
	}
	return false
}
