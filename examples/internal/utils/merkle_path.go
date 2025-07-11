package utils

import (
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// PrintMerklePath dumps every element exactly as it comes from the SDK.
func PrintMerklePath(path *transaction.MerklePath) {
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

// PrintMerklePathInfo prints the metadata that GetMerklePath returns.
func PrintMerklePathInfo(r *wdk.MerklePathResult) {
	fmt.Printf("service,%s\n", r.Name)

	if bh := r.BlockHeader; bh != nil {
		fmt.Printf("block_hash,%s\n", bh.Hash)
		fmt.Printf("block_height,%d\n", bh.Height)
		fmt.Printf("merkle_root,%s\n", bh.MerkleRoot)
	}

	for _, n := range r.Notes {
		fmt.Printf("note,%s,%s\n", n.What, n.When.Format(time.RFC3339))
	}
}
