package party

import (
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// WalletParty contain information about user and storage party identities and beef party
type WalletParty struct {
	UserParty    string
	StorageParty string
	BeefParty    *wdk.BeefParty
}

// GetKnownTxIDs merges new known transaction IDs into the wallet's known transactions and return list of known txIDs.
func (wp *WalletParty) GetKnownTxIDs(newKnownTxIDs ...chainhash.Hash) (primitives.TXIDHexStrings, error) {
	for _, txID := range newKnownTxIDs {
		wp.BeefParty.MergeTxidOnly(&txID)
	}

	result := wp.BeefParty.ValidateTransactions()

	hexStrings := make([]primitives.TXIDHexString, 0, len(result.Valid))
	for _, id := range result.Valid {
		hexStrings = append(hexStrings, primitives.TXIDHexString(id))
	}

	return hexStrings, nil
}
