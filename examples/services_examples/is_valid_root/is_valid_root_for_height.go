// examples/is_valid_root/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/chainhash"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
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
		panic(fmt.Errorf("failed to parse root hex %s: %w", rootHex, err))
	}

	ok, err := srv.IsValidRootForHeight(context.Background(), root, height)
	if err != nil {
		panic(fmt.Errorf("IsValidRootForHeight failed: %w", err))
	}

	show.IsValidRootForHeightOutput(height, rootHex, ok)
}

// Output:
// Is valid root for height:
// Height  Merkle Root (hex)  Valid
// 903321  559ce1f8394df2f008a9c4d23e71256c999ea05aba47e8620ab66f1f24c8a0fd  true
//
// Height: 903321 | Merkle Root: 559ce1f8394df2f008a9c4d23e71256c999ea05aba47e8620ab66f1f24c8a0fd | Valid: true