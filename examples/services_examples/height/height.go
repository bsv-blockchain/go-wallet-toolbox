package main

import (
	"context"
	"fmt"
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
	show.ProcessStart("Get Height")

	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
	cfg.BHS.APIKey = "..." // use default api key DefaultAppToken from the BHS service https://github.com/bsv-blockchain/block-headers-service/blob/main/config/defaults.go#L8

	srv := services.New(slog.Default(), cfg)
	show.Step("Wallet-Services", "fetching main-chain height (BHS → WoC → Bitails fallback)")

	height, err := srv.CurrentHeight(context.Background())
	if err != nil {
		show.Error(fmt.Sprintf("failed to get current height: %v", err))
		return
	}

	show.Success("Fetched chain tip height")
	show.CurrentHeightOutput(height)
	show.ProcessComplete("Get Height")
}

/* Output:
🚀 STARTING: Get Height
============================================================

=== STEP ===
Wallet-Services is performing: fetching main-chain height (BHS → WoC → Bitails fallback)
--------------------------------------------------
2025/07/14 10:47:42 WARN error when calling service service=services.GetHeight service.name=BlockHeadersService error="failed for service BlockHeadersService: unexpected HTTP 401 for http://localhost:8080/api/v1/chain/tip/longest"
✅ SUCCESS: Fetched chain tip height

Get Height: 905465
============================================================
🎉 COMPLETED: Get Height
*/
