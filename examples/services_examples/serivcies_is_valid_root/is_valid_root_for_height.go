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

	show.ProcessStart("Is Valid Root For Height")
	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
	cfg.BHS.APIKey = "..." // use default api key DefaultAppToken from the BHS service https://github.com/bsv-blockchain/block-headers-service/blob/main/config/defaults.go#L8

	srv := services.New(slog.Default(), cfg)

	root, err := chainhash.NewHashFromHex(rootHex)
	if err != nil {
		panic(fmt.Errorf("failed to parse root hex %s: %w", rootHex, err))
	}

	show.Step("Wallet-Services", fmt.Sprintf("checking if root %s is valid for height %d", rootHex, height))
	ok, err := srv.IsValidRootForHeight(context.Background(), root, height)
	if err != nil {
		panic(fmt.Errorf("IsValidRootForHeight failed: %w", err))
	}

	show.Success("Checked if root is valid for height")
	show.IsValidRootForHeightOutput(height, rootHex, ok)
	show.ProcessComplete("Is Valid Root For Height")
}

/* Output:
🚀 STARTING: Is Valid Root For Height
============================================================

=== STEP ===
Wallet-Services is performing: checking if root 559ce1f8394df2f008a9c4d23e71256c999ea05aba47e8620ab66f1f24c8a0fd is valid for height 903321
--------------------------------------------------
✅ SUCCESS: Checked if root is valid for height

Height: 903321 | Merkle Root: 559ce1f8394df2f008a9c4d23e71256c999ea05aba47e8620ab66f1f24c8a0fd | Valid: true
============================================================
🎉 COMPLETED: Is Valid Root For Height
*/
