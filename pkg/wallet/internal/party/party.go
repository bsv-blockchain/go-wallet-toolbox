package party

import (
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// WalletParty contains information about user and storage party identities and beef party.
type WalletParty struct {
	UserParty    string
	StorageParty string
	BeefParty    *wdk.BeefParty
}

// GetKnownTxIDs merges new known transaction IDs into the wallet's known transactions and returns a list of known txIDs.
func (wp *WalletParty) GetKnownTxIDs(newKnownTxIDs ...chainhash.Hash) (primitives.TXIDHexStrings, error) {
	// Locked BeefParty wrappers: the shared Beef graph is mutated by every
	// concurrent action on this wallet.
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
