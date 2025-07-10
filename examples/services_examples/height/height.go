package main

import (
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

/*
when you call srv.Height() the wallet-services
stack:
1. Asks the Block-Headers Service (/chain/tip/longest) for the current blocks value.
2. If that fails, asks WhatsOnChain (/chain/info) for the current blocks value.
3. If that fails, falls back to Bitails (/network/info).
4. Returns the first non-zero height it obtains.

So the number printed under Current Tip Height is simply "how many blocks are
currently in the main BSV chain (mainnet) right now."
*/
func main() {
	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)

	cfg.BHS.APIKey = "..." // use default api key DefaultAppToken from the BHS service https://github.com/bsv-blockchain/block-headers-service/blob/main/config/defaults.go#L8

	srv := services.New(slog.Default(), cfg)

	height := srv.Height()

	show.HeightOutput(height)
}

// Output:
// Get Height:
// Current Tip Height
// 903321
//
// Current Tip Height: 903321