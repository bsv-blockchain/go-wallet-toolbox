// examples/is_valid_root/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bsv-blockchain/go-sdk/chainhash"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

func main() {
	const (
		height = uint32(903321)
		// https://whatsonchain.com/block-height/903321?tab=json
		rootHex = "559ce1f8394df2f008a9c4d23e71256c999ea05aba47e8620ab66f1f24c8a0fd"
	)

	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
	srv := services.New(slog.Default(), cfg)

	root, err := chainhash.NewHashFromHex(rootHex)
	if err != nil {
		panic(fmt.Sprintf("failed to parse root hex %s: %v", rootHex, err))
	}

	ok, err := srv.IsValidRootForHeight(context.Background(), root, height)
	if err != nil {
		panic(fmt.Sprintf("IsValidRootForHeight failed: %v", err))
	}

	headers := []string{"Height", "Merkle Root (hex)", "Valid"}
	rows := [][]string{
		{fmt.Sprint(height), rootHex, fmt.Sprint(ok)},
	}
	printTable(headers, rows)
}

func printTable(headers []string, rows [][]string) {
	colW := make([]int, len(headers))
	for i, h := range headers {
		colW[i] = len(h)
	}
	for _, r := range rows {
		for i, cell := range r {
			if len(cell) > colW[i] {
				colW[i] = len(cell)
			}
		}
	}

	printRow := func(cells []string) {
		for i, c := range cells {
			fmt.Printf("%-*s  ", colW[i], c)
		}
		fmt.Println()
	}

	printRow(headers)
	for i := range headers {
		fmt.Printf("%s  ", strings.Repeat("-", colW[i]))
	}
	fmt.Println()
	for _, r := range rows {
		printRow(r)
	}
}
