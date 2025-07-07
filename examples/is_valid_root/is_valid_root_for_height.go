// examples/is_valid_root/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/bsv-blockchain/go-sdk/chainhash"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services"
)

func main() {
	const (
		height  = uint32(903321)
		baseURL = "https://api.whatsonchain.com/v1/bsv/main"
	)

	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
	ws := services.New(slog.Default(), cfg)

	rootHex, err := fetchRoot(baseURL, height)
	dieIf(err, "fetch header")

	root, err := chainhash.NewHashFromHex(rootHex)
	dieIf(err, "NewHashFromHex")

	ok, err := ws.IsValidRootForHeight(context.Background(), root, height)
	dieIf(err, "IsValidRootForHeight")

	bad := *root
	bad[0] ^= 0xff
	okBad, err := ws.IsValidRootForHeight(context.Background(), &bad, height)
	dieIf(err, "IsValidRootForHeight (bit-flipped)")

	headers := []string{"Height", "Merkle Root (hex)", "Valid"}
	rows := [][]string{
		{fmt.Sprint(height), rootHex, fmt.Sprint(ok)},
		{fmt.Sprint(height), bad.String(), fmt.Sprint(okBad) + " (bit-flipped)"},
	}
	printTable(headers, rows)
}

func fetchRoot(base string, h uint32) (string, error) {
	url := fmt.Sprintf("%s/block/%d/header", base, h)
	res, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("WoC status %d", res.StatusCode)
	}
	var dto struct {
		MerkleRoot string `json:"merkleroot"`
	}
	b, _ := io.ReadAll(res.Body)
	if err := json.Unmarshal(b, &dto); err != nil {
		return "", err
	}
	return dto.MerkleRoot, nil
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
		fmt.Printf("%s  ", stringRepeat("-", colW[i]))
	}
	fmt.Println()
	for _, r := range rows {
		printRow(r)
	}
}

func stringRepeat(ch string, n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("%s%s", ch, stringRepeat(ch, n-1))
}

func dieIf(err error, what string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", what, err)
		os.Exit(1)
	}
}
