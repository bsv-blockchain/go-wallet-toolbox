// examples/find_chain_tip/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

func main() {
	show.ProcessStart("Find Chain Tip Header")

	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
	cfg.BHS.URL = "http://localhost:8080/api/v1"
	cfg.BHS.APIKey = ".......your_api_key_here..."
	svc := services.New(slog.Default(), cfg)
	ctx := context.Background()

	show.Step("FindChainTipHeader", "Finds the latest block header in the longest chain")
	tip, err := svc.FindChainTipHeader(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to find chain tip header: %w", err))
	}

	show.Success("Fetched chain tip header")
	show.ChainTipHeaderOutput(tip)

	show.ProcessComplete("Find Chain Tip Header")
}

/* Output:
🚀 STARTING: Find Chain Tip Header
============================================================

=== STEP ===
FindChainTipHeader is performing: Finds the latest block header in the longest chain
--------------------------------------------------
✅ SUCCESS: Fetched chain tip header
Chain Tip Header:
Height  Hash                                                              Version   Prev-Hash                                                         Merkle-Root                                                       Time        Bits      Nonce
------  ----------------------------------------------------------------  --------  ----------------------------------------------------------------  ----------------------------------------------------------------  ----------  --------  ---------
905604  000000000000000005698beb20b1d7ff4ad1860314bd3c395c6db123f91c7ffd  283e2000  00000000000000000e9ee9c173a140cdc20e7f9f9f708ee276a9922c4fd6dea3  5ab8bf3278ab9d2912ade1260cacd5df9ee0b78670bbc87b9fb05a7ea5755b90  1752570909  1817a94f  342927395
============================================================
🎉 COMPLETED: Find Chain Tip Header
*/
