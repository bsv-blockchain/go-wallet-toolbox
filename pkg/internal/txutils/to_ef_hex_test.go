package txutils

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seq2"
)

// validateSingleLeafTxOld is the original implementation kept here purely for benchmarking comparison.
func validateSingleLeafTxOld(beef *transaction.Beef) error {
	idToTx := seq2.FromMap(beef.Transactions)

	inDegree := seq2.CollectToMap(seq2.MapValues(idToTx, func(tx *transaction.BeefTx) int { return 0 }))

	inputs := seq.FlattenSlices(seq.Map(seq2.Values(idToTx), func(tx *transaction.BeefTx) []*transaction.TransactionInput {
		if tx.Transaction == nil {
			return nil
		}
		return tx.Transaction.Inputs
	}))

	inputsIds := seq.Map(inputs, func(input *transaction.TransactionInput) chainhash.Hash {
		if input.SourceTXID == nil {
			return chainhash.Hash{}
		}
		return *input.SourceTXID
	})

	seq.ForEach(inputsIds, func(inputTxID chainhash.Hash) {
		if _, ok := inDegree[inputTxID]; !ok {
			return
		}
		inDegree[inputTxID]++
	})

	txIDsWithoutChildren := seq2.FilterByValue(seq2.FromMap(inDegree), is.Zero)

	subjectTxs := seq.Collect(seq2.Keys(txIDsWithoutChildren))
	if len(subjectTxs) != 1 {
		return nil // we don't care about the error in the bench, just the work
	}
	return nil
}

// buildLinearChain creates a BEEF containing a chain of n transactions where each spends the previous one.
func buildLinearChain(n int) *transaction.Beef {
	beef := transaction.NewBeefV2()
	var prevTxID chainhash.Hash
	for i := 0; i < n; i++ {
		tx := transaction.NewTransaction()
		if i > 0 {
			tx.Inputs = append(tx.Inputs, &transaction.TransactionInput{
				SourceTXID:       &prevTxID,
				SourceTxOutIndex: 0,
			})
		}
		// Minimal valid output so TxID() is stable and Merge works
		tx.Outputs = append(tx.Outputs, &transaction.TransactionOutput{
			Satoshis:      1000,
			LockingScript: script.NewFromBytes([]byte{0x51}), // OP_1
		})
		btx, err := beef.MergeTransaction(tx)
		if err != nil {
			panic(err)
		}
		prevTxID = *btx.Transaction.TxID()
	}
	return beef
}

// buildIndependentLeaves creates a BEEF with k independent root transactions (k leaves).
func buildIndependentLeaves(k int) *transaction.Beef {
	beef := transaction.NewBeefV2()
	for i := 0; i < k; i++ {
		tx := transaction.NewTransaction()
		tx.Outputs = append(tx.Outputs, &transaction.TransactionOutput{
			Satoshis:      1000,
			LockingScript: script.NewFromBytes([]byte{0x51}),
		})
		_, err := beef.MergeTransaction(tx)
		if err != nil {
			panic(err)
		}
	}
	return beef
}

func BenchmarkValidateSingleLeafTx(b *testing.B) {
	cases := []struct {
		name string
		beef *transaction.Beef
	}{
		{"chain-1", buildLinearChain(1)},
		{"chain-4", buildLinearChain(4)},
		{"chain-16", buildLinearChain(16)},
		{"chain-64", buildLinearChain(64)},
		{"leaves-2", buildIndependentLeaves(2)},
		{"leaves-8", buildIndependentLeaves(8)},
	}

	for _, tc := range cases {
		b.Run(tc.name+"/old", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = validateSingleLeafTxOld(tc.beef)
			}
		})
		b.Run(tc.name+"/new", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = ValidateSingleLeafTx(tc.beef)
			}
		})
	}
}
