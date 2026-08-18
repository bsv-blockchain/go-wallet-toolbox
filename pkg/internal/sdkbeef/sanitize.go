// Package sdkbeef contains the BEEF handling the toolbox has to do on top of
// go-sdk's, because go-sdk's BEEF reader leaves a parsed graph in a state the
// rest of its own API cannot handle.
package sdkbeef

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
)

// Sanitize repairs a Beef that came out of go-sdk's reader.
//
// readBeefTx allocates every entry as `BeefTx{DataFormat: f, Transaction:
// &Transaction{}}` and, for a TxIDOnly entry, never fills that transaction in.
// So a round trip through the wire format turns a bare txid into a bare txid
// PLUS an empty transaction object (version 0, no inputs, no outputs), and
// readBeefTxFull then wires it into any later transaction that spends it -
// `input.SourceTransaction = sourceObj.Transaction` with no check on the
// source's data format. Three things follow, all of them observed:
//
//   - transaction.TransactionInput.SourceTxOutput indexes Outputs without a
//     bounds check, so script verification panics on the empty parent;
//   - Beef.MergeTransaction walks SourceTransaction pointers, so the empty
//     transaction gets merged as a graph entry in its own right, under the txid
//     of 10 zero bytes (f702453d...). That is the "floating empty transaction"
//     that shows up in stored BEEFs once bare txids are in play;
//   - anything that treats a non-nil SourceTransaction as "already hydrated"
//     (see txutils.HydrateBEEF) keeps the empty parent instead of the real one.
//
// Sanitize drops the placeholders, un-wires every input pointing at one, and
// removes any empty transaction already merged as an entry. A transaction with
// no inputs and no outputs is not a valid BSV transaction, so removing them
// cannot discard real data.
//
// Call it on every Beef built from bytes the process did not construct itself.
func Sanitize(beef *transaction.Beef) {
	if beef == nil {
		return
	}

	for txIDHash, beefTx := range beef.Transactions {
		switch {
		case beefTx.DataFormat == transaction.TxIDOnly:
			beefTx.Transaction = nil
			if beefTx.KnownTxID == nil {
				known := txIDHash
				beefTx.KnownTxID = &known
			}
		case isEmpty(beefTx.Transaction):
			delete(beef.Transactions, txIDHash)
		}
	}

	for _, beefTx := range beef.Transactions {
		if beefTx.Transaction == nil {
			continue
		}
		for _, input := range beefTx.Transaction.Inputs {
			if isEmpty(input.SourceTransaction) {
				input.SourceTransaction = nil
			}
		}
	}
}

func isEmpty(tx *transaction.Transaction) bool {
	return tx != nil && len(tx.Inputs) == 0 && len(tx.Outputs) == 0
}

// ParseBytes parses BEEF (or atomic BEEF) bytes into a sanitized Beef.
func ParseBytes(raw []byte) (*transaction.Beef, error) {
	beef, err := transaction.NewBeefFromBytes(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse beef bytes: %w", err)
	}

	Sanitize(beef)

	return beef, nil
}

// MergeBytes merges BEEF bytes into dst and sanitizes the result.
func MergeBytes(dst *transaction.Beef, raw []byte) error {
	if err := dst.MergeBeefBytes(raw); err != nil {
		return fmt.Errorf("failed to merge beef bytes: %w", err)
	}

	Sanitize(dst)

	return nil
}
