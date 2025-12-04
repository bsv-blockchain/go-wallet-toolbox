package party

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// VerifyReturnedTxIDOnlyBeef verify for not known TxIDOnly txs
func VerifyReturnedTxIDOnlyBeef(bp *wdk.BeefParty, beef primitives.BEEF) (primitives.BEEF, error) {
	b, err := transaction.NewBeefFromBytes(beef)
	if err != nil {
		return nil, fmt.Errorf("failed to create beef from bytes: %w", err)
	}

	b, err = verifyReturnedTxIDOnly(bp, b)
	if err != nil {
		return nil, fmt.Errorf("failed to verify returned beef txid only: %w", err)
	}

	bytes, err := b.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get bytes from beef: %w", err)
	}

	return bytes, nil
}

// VerifyReturnedTxIDOnlyAtomicBEEF verify for not known TxIDOnly txs
func VerifyReturnedTxIDOnlyAtomicBEEF(bp *wdk.BeefParty, txID chainhash.Hash, beef primitives.BEEF, knownTxIDs ...primitives.TXIDHexString) (primitives.BEEF, error) {
	b, err := transaction.NewBeefFromBytes(beef)
	if err != nil {
		return nil, fmt.Errorf("failed to create beef from bytes: %w", err)
	}

	b, err = verifyReturnedTxIDOnly(bp, b, knownTxIDs...)
	if err != nil {
		return nil, fmt.Errorf("failed to verify returned beef txid only: %w", err)
	}

	bytes, err := b.AtomicBytes(&txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bytes from beef: %w", err)
	}

	return bytes, nil
}

func verifyReturnedTxIDOnly(beefParty *wdk.BeefParty, beef *transaction.Beef, knownTxIDs ...primitives.TXIDHexString) (*transaction.Beef, error) {
	for _, btx := range beef.Transactions {
		if btx.DataFormat != transaction.TxIDOnly {
			continue
		}
		tx := beefParty.FindAtomicTransactionByHash(btx.KnownTxID)
		if tx == nil {
			return nil, fmt.Errorf(" tx with only txid not found in beef party: %s", btx.KnownTxID.String())
		}

		_, err := beef.MergeTransaction(tx)
		if err != nil {
			return nil, fmt.Errorf("failed to merge transaction with only txid: %w", err)
		}
	}

	for _, btx := range beef.Transactions {
		txIDHexString := primitives.TXIDHexString(btx.Transaction.TxID().String())
		if knownTxIDs != nil && contains(knownTxIDs, txIDHexString) {
			continue
		}

		if btx.DataFormat == transaction.TxIDOnly {
			return nil, fmt.Errorf("remaining txidOnly %s is not known", btx.KnownTxID.String())
		}
	}

	return beef, nil
}

func contains(slice []primitives.TXIDHexString, s primitives.TXIDHexString) bool {
	for _, v := range slice {
		if v.String() == s.String() {
			return true
		}
	}
	return false
}
