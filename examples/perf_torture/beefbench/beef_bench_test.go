// Package beefbench reproduces the hot paths from the production flame graph
// (transaction.(*Beef).MergeTransaction recursion with ComputeRoot /
// GetOffsetLeaf / Sha256 towers) as pure in-memory Go benchmarks, so BEEF
// performance work can be iterated on in seconds instead of waiting for
// mainnet blocks.
//
// Run:
//
//	go test -bench . -benchmem ./examples/perf_torture/beefbench
//
// Capture a flame graph of a single benchmark:
//
//	go test -bench 'MergeTransaction_ChainDepth/depth=100' -benchtime 10x \
//	    -cpuprofile cpu.out ./examples/perf_torture/beefbench
//	go tool pprof -http=:8080 cpu.out
package beefbench

import (
	"fmt"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

// buildChain builds a chain of `depth` unmined transactions where tx i spends
// tx i-1, rooted at a mined transaction carrying a merkle path — the exact
// shape the wallet produces when a single change UTXO is respent over and
// over between blocks (change_basket.number_of_desired_utxos = 1).
// Returns the unmined tip.
func buildChain(tb testing.TB, depth int) *transaction.Transaction {
	tb.Helper()

	lock := &script.Script{}
	if err := lock.AppendOpcodes(script.OpTRUE); err != nil {
		tb.Fatal(err)
	}

	root := transaction.NewTransaction()
	root.AddOutput(&transaction.TransactionOutput{Satoshis: 1_000_000, LockingScript: lock})
	root.MerklePath = singleTxPath(root.TxID(), 800_000)

	prev := root
	for i := 0; i < depth; i++ {
		tx := transaction.NewTransaction()
		tx.AddInput(&transaction.TransactionInput{
			SourceTXID:        prev.TxID(),
			SourceTxOutIndex:  0,
			SourceTransaction: prev,
		})
		tx.AddOutput(&transaction.TransactionOutput{
			Satoshis:      prev.Outputs[0].Satoshis - 1,
			LockingScript: lock,
		})
		prev = tx
	}
	return prev
}

// singleTxPath builds a minimal BUMP proving txid in a two-leaf block.
func singleTxPath(txid *chainhash.Hash, blockHeight uint32) *transaction.MerklePath {
	isTxid := true
	sibling := chainhash.DoubleHashH([]byte("sibling"))
	return transaction.NewMerklePath(blockHeight, [][]*transaction.PathElement{{
		{Offset: 0, Hash: txid, Txid: &isTxid},
		{Offset: 1, Hash: &sibling},
	}})
}

// compoundPath builds a BUMP with `leaves` row-0 leaves (all txid-flagged),
// mimicking the combined bumps the wallet accumulates: Combine() drops parent
// nodes, so ComputeRoot must rebuild every interior node via GetOffsetLeaf.
func compoundPath(tb testing.TB, leaves int, blockHeight uint32) *transaction.MerklePath {
	tb.Helper()
	if leaves&(leaves-1) != 0 {
		tb.Fatalf("leaves must be a power of two, got %d", leaves)
	}
	isTxid := true
	row := make([]*transaction.PathElement, leaves)
	for i := range row {
		h := chainhash.DoubleHashH([]byte{byte(i), byte(i >> 8), 0xbe, 0xef})
		row[i] = &transaction.PathElement{Offset: uint64(i), Hash: &h, Txid: &isTxid}
	}
	return transaction.NewMerklePath(blockHeight, [][]*transaction.PathElement{row})
}

// BenchmarkMergeTransaction_ChainDepth measures one MergeTransaction of an
// unmined chain tip into a fresh BEEF — the per-createAction cost the client
// pays at a given unmined chain depth. Expect superlinear wall-clock growth
// across depths (uncached TxID() re-serializes + double-sha256 per level).
func BenchmarkMergeTransaction_ChainDepth(b *testing.B) {
	for _, depth := range []int{10, 25, 50, 100, 200} {
		tip := buildChain(b, depth)
		b.Run(benchName("depth", depth), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				beef := transaction.NewBeefV2()
				if _, err := beef.MergeTransaction(tip); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkMergeTransaction_Repeated merges the SAME tip into an accumulated
// BEEF over and over — the wallet-side BeefParty scenario. MergeTransaction
// has no "already present" fast path (it removes and re-adds), so every merge
// re-walks the full ancestry.
func BenchmarkMergeTransaction_Repeated(b *testing.B) {
	const depth = 50
	tip := buildChain(b, depth)
	beef := transaction.NewBeefV2()
	if _, err := beef.MergeTransaction(tip); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := beef.MergeTransaction(tip); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBeefBytes measures serialization of a deep-chain BEEF —
// Beef.Bytes() calls uncached TxID() once more per contained transaction.
func BenchmarkBeefBytes(b *testing.B) {
	tip := buildChain(b, 100)
	beef := transaction.NewBeefV2()
	if _, err := beef.MergeTransaction(tip); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := beef.Bytes(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkComputeRoot_CompoundLeaves measures a single ComputeRoot over a
// row-0-heavy compound bump — GetOffsetLeaf rebuilds the whole interior tree
// with sha256d per node and memoizes nothing between calls.
func BenchmarkComputeRoot_CompoundLeaves(b *testing.B) {
	for _, leaves := range []int{64, 256, 1024} {
		bump := compoundPath(b, leaves, 800_000)
		b.Run(benchName("leaves", leaves), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := bump.ComputeRoot(nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkMergeBump_SameHeight measures merging a bump into a BEEF that
// already holds another bump for the same block height: MergeBump runs
// ComputeRoot on BOTH bumps to compare roots, then Combine recomputes both
// again — four uncached root computations per pair.
func BenchmarkMergeBump_SameHeight(b *testing.B) {
	const leaves = 256
	existing := compoundPath(b, leaves, 800_000)
	incoming := compoundPath(b, leaves, 800_000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		beef := transaction.NewBeefV2()
		beef.MergeBump(existing)
		beef.MergeBump(incoming)
	}
}

func benchName(key string, v int) string {
	return fmt.Sprintf("%s=%d", key, v)
}
