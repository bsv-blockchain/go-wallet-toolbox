package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

/*
Example output for a successful Merkle Path retrieval:
{
    "hash": "000000000000000004f576c9cdc2b0ee65f04c3f03c08529c380d6a76d262641",
    "confirmations": 1,
    "size": 3072008,
    "height": 903321,
    "version": 771751936,
    "versionHex": "2e000000",
    "merkleroot": "559ce1f8394df2f008a9c4d23e71256c999ea05aba47e8620ab66f1f24c8a0fd",
    "txcount": 2081,
    "time": 1751198131,
    "mediantime": 1751193155,
    "nonce": 1179404358,
    "bits": "181606c9",
    "difficulty": 49916901237.38432,
    "chainwork": "00000000000000000000000000000000000000000166fb665858a1d5a2a1d5ab",
    "previousblockhash": "00000000000000001523ba5d75d66b2f8314c2d7a2476d8a19c8e84ab51c53bd",
    "nextblockhash": "",
    "coinbaseTx": {
        "hex": "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff170399c80d2f43555656452f0150cbfa27d51703e1a32500ffffffff01f3d4a112000000001976a914d648686cf603c11850f39600e37312738accca8f88ac00000000",
        "txid": "9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6",
        "hash": "9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6",
        "size": 108,
        "version": 1,
        "locktime": 0,
        "vin": [
            {
                "n": 0,
                "coinbase": "0399c80d2f43555656452f0150cbfa27d51703e1a32500",
                "sequence": 4294967295,
                "minerInfo": {
                    "name": "CUVVE",
                    "type": "tag"
                }
            }
        ],
        "vout": [
            {
                "value": 3.12595699,
                "n": 0,
                "scriptPubKey": {
                    "asm": "OP_DUP OP_HASH160 d648686cf603c11850f39600e37312738accca8f OP_EQUALVERIFY OP_CHECKSIG",
                    "hex": "76a914d648686cf603c11850f39600e37312738accca8f88ac",
                    "reqSigs": 1,
                    "type": "pubkeyhash",
                    "addresses": [
                        "1LY2M3RCkEVKo82ym1SQ1iZGQhM5Lf5Pkf"
                    ],
                    "isTruncated": false
                },
                "scripthash": "020a5314df44ccfa5b8e5c5c5b354f397d2590832c40e032099f442b12fca370"
            }
        ],
        "blockhash": "000000000000000004f576c9cdc2b0ee65f04c3f03c08529c380d6a76d262641",
        "confirmations": 1,
        "time": 1751198131,
        "blocktime": 1751198131,
        "vincount": 1,
        "voutcount": 1,
        "voutvalue": 3.12595699
    },
    "totalFees": 0.0009569900000001574
}
*/

// processes services one by one until a successful result is obtained.
// The context and argument is passed to each service.
// Returns the first successful result or an error if all services fail.
// https://whatsonchain.com/block-height/903321?tab=json <-- Example of a block with Merkle Path
func main() {
	txID := "9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6"
	network := defs.NetworkMainnet

	serviceCfg := defs.DefaultServicesConfig(network)
	walletServices := services.New(slog.Default(), serviceCfg)

	result, err := walletServices.MerklePath(context.Background(), txID)
	if err != nil {
		panic(fmt.Errorf("failed to get MerklePath: %w", err))
	}

	printMetadata(result, txID)
	fmt.Print("\n\n")
	printMerklePath(result.MerklePath, "Merkle Path for txID: "+txID)
}

func renderTable(title string, headers []string, rows [][]string) {
	if title != "" {
		fmt.Printf("%s\n", title)
	}

	colWidth := make([]int, len(headers))
	for i, h := range headers {
		colWidth[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if l := len(cell); l > colWidth[i] {
				colWidth[i] = l
			}
		}
	}

	printRow := func(cells []string) {
		for i, cell := range cells {
			fmt.Printf("%-*s  ", colWidth[i], cell)
		}
		fmt.Println()
	}

	printRow(headers)

	for i := range headers {
		fmt.Printf("%s  ", strings.Repeat("-", colWidth[i]))
	}
	fmt.Println()

	for _, row := range rows {
		printRow(row)
	}
}

func printMetadata(result *wdk.MerklePathResult, txID string) {
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

	renderTable("Merkle Path Metadata", []string{"Field", "Value"}, rows)
}

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

	renderTable(
		fmt.Sprintf("Block Height: %d | %s", tbl.BlockHeight, title),
		[]string{"Level", "Offset", "Hash", "Is TxID"},
		rows,
	)
}
