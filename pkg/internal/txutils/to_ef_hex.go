package txutils

import (
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/transaction"
)

// ToEFHex converts a BEEF to EF hex format for the subject transaction.
// It finds the subject tx (the one without children), hydrates it, and converts to EF.
// func ToEFHex(beef *transaction.Beef, logger *slog.Logger) (string, error) {
// 	// This is a temporary fix on BEEF until the merge beef will work properly and will bind the tx with bump.
// 	BindBumpsAndTransactions(beef, logger)
//
// 	// This is a temporary solution until go-sdk properly implements BEEF serialization
// 	// It searches for the subject transaction in transaction.Beef and serializes this one to EF hex.
// 	// For now, it's not supporting more than one subject transaction.
// 	idToTx := seq2.FromMap(beef.Transactions)
//
// 	// inDegree will contain the number of transactions for which the given tx is a parent
// 	inDegree := seq2.CollectToMap(seq2.MapValues(idToTx, func(tx *transaction.BeefTx) int { return 0 }))
//
// 	// txsNotMined we are not interested in inputs of mined transactions
// 	txsNotMined := seq.Filter(seq2.Values(idToTx), func(tx *transaction.BeefTx) bool {
// 		return tx.Transaction.MerklePath == nil
// 	})
//
// 	inputs := seq.FlattenSlices(seq.Map(txsNotMined, func(tx *transaction.BeefTx) []*transaction.TransactionInput {
// 		return tx.Transaction.Inputs
// 	}))
//
// 	inputsIds := seq.Map(inputs, func(input *transaction.TransactionInput) chainhash.Hash {
// 		return *input.SourceTXID
// 	})
//
// 	seq.ForEach(inputsIds, func(inputTxID chainhash.Hash) {
// 		if _, ok := inDegree[inputTxID]; !ok {
// 			panic(fmt.Sprintf("unexpected input txid %s, this shouldn't ever happen", inputTxID))
// 		}
// 		inDegree[inputTxID]++
// 	})
//
// 	txIDsWithoutChildren := seq2.FilterByValue(seq2.FromMap(inDegree), is.Zero)
//
// 	subjectTxs := seq.Collect(seq2.Keys(txIDsWithoutChildren))
// 	if len(subjectTxs) != 1 {
// 		return "", fmt.Errorf("expected only one subject tx, but got %d", len(subjectTxs))
// 	}
//
// 	subjectTx, ok := beef.Transactions[subjectTxs[0]]
// 	if !ok {
// 		return "", fmt.Errorf("expected to find subject tx %s in beef, but it was not found, this shouldn't ever happen", subjectTxs[0])
// 	}
//
// 	// Another temporary workaround until go-sdk properly implements BEEF serialization
// 	err := HydrateTransactionFromBEEF(subjectTx.Transaction, beef)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to rebuild subject tx: %w", err)
// 	}
//
// 	efHex, err := subjectTx.Transaction.EFHex()
// 	if err != nil {
// 		return "", fmt.Errorf("failed to convert subject tx into EF hex: %w", err)
// 	}
//
// 	return efHex, nil
// }

// TxToEFHex converts a specific transaction to EF hex, using BEEF for hydration.
// Call BindBumpsAndTransactions once before calling this in a loop.
func TxToEFHex(tx *transaction.Transaction, beef *transaction.Beef) (string, error) {
	err := HydrateTransactionFromBEEF(tx, beef)
	if err != nil {
		return "", err
	}
	return tx.EFHex()
}

// BindBumpsAndTransactions binds BUMPs to transactions in BEEF.
func BindBumpsAndTransactions(beef *transaction.Beef, logger *slog.Logger) {
	for i, bump := range beef.BUMPs {
		if len(bump.Path) == 0 || len(bump.Path[0]) == 0 {
			logger.Warn("got bump without bottom path", slog.String("merklePath", bump.Hex()))
			continue
		}
		for _, element := range bump.Path[0] {
			if element.Txid != nil && *element.Txid {
				if element.Hash == nil {
					logger.Error("got leaf marked as txid in BUMP but hash is nil")
					continue
				}
				tx, ok := beef.Transactions[*element.Hash]
				if !ok {
					logger.Warn("got leaf marked as txid in BUMP that is not part of the BEEF", slog.String("txid", element.Hash.String()))
					continue
				}
				tx.BumpIndex = i
				tx.DataFormat = transaction.RawTxAndBumpIndex
				tx.Transaction.MerklePath = bump
			}
		}
	}
}
