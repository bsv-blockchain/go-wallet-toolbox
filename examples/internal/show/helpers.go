package show

import (
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func printMerklePath(tbl *transaction.MerklePath, title string) {
	var rows [][]string

	for lvl, elems := range tbl.Path {
		for _, el := range elems {
			isTx := ""
			if el.Txid != nil && *el.Txid {
				isTx = "yes"
			}
			rows = append(rows, []string{
				fmt.Sprint(lvl),
				fmt.Sprint(el.Offset),
				el.Hash.String(),
				isTx,
			})
		}
	}

	PrintTable(
		fmt.Sprintf("Block Height: %d | %s", tbl.BlockHeight, title),
		[]string{"Level", "Offset", "Hash", "Is TxID"},
		rows,
	)
}

func displayMerklePathInfo(result *wdk.MerklePathResult, txID string) {
	var rows [][]string

	rows = append(rows, []string{"Service", result.Name})

	if result.BlockHeader != nil {
		rows = append(rows,
			[]string{"Block Hash", result.BlockHeader.Hash},
			[]string{"Block Height", fmt.Sprint(result.BlockHeader.Height)},
			[]string{"Merkle Root", result.BlockHeader.MerkleRoot},
		)
	}

	for _, note := range result.Notes {
		rows = append(rows,
			[]string{"Note", fmt.Sprintf("%s at %s", note.What, note.When.Format(time.RFC3339))},
		)
	}

	root, err := result.MerklePath.ComputeRootHex(&txID)
	if err != nil {
		rows = append(rows, []string{"Computed Merkle Root", fmt.Sprintf("ERROR: %v", err)})
	} else {
		rows = append(rows, []string{"Computed Merkle Root", root})
	}

	PrintTable("Merkle Path Metadata:", []string{"Field", "Value"}, rows)
}