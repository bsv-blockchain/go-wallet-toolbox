package show

import (
	"fmt"
	"os"

	"github.com/bsv-blockchain/go-sdk/transaction"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// printMerklePath dumps every element exactly as it comes from the SDK.
func printMerklePath(path *transaction.MerklePath) {
	for lvl, elems := range path.Path {
		for _, el := range elems {
			isTx := el.Txid != nil && *el.Txid
			fmt.Printf("%d,%d,%s,%t\n",
				lvl,
				el.Offset,
				el.Hash.String(),
				isTx,
			)
		}
	}
}

// printMerklePathInfo prints the metadata that GetMerklePath returns.
func printMerklePathInfo(r *wdk.MerklePathResult) {
	fmt.Printf("service: %s\n", r.Name)

	if bh := r.BlockHeader; bh != nil {
		fmt.Printf("block_hash: %s\n", bh.Hash)
		fmt.Printf("block_height: %d\n", bh.Height)
		fmt.Printf("merkle_root: %s\n", bh.MerkleRoot)
	}

	if r.Notes != nil {
		if err := r.Notes.PrettyPrint(os.Stdout); err != nil {
			panic(fmt.Errorf("failed to pretty print notes: %w", err))
		}
	}
}
