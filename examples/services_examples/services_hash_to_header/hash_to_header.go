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
	const blockHash = "000000000000000004a288072ebb35e37233f419918f9783d499979cb6ac33eb" // example block hash

	show.ProcessStart("Hash To Header")

	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)

	srv := services.New(slog.Default(), cfg)

	show.Step("Wallet-Services", fmt.Sprintf("fetching block header for hash %s", blockHash))
	header, err := srv.HashToHeader(context.Background(), blockHash)
	if err != nil {
		panic(fmt.Errorf("failed to get header for hash: %w", err))
	}

	show.Success("Fetched block header from hash")
	show.ChainTipHeaderOutput(header)
	show.ProcessComplete("Hash To Header")
}

/* Output:

🚀 STARTING: Hash To Header
============================================================

=== STEP ===
Wallet-Services is performing: fetching block header for hash 000000000000000004a288072ebb35e37233f419918f9783d499979cb6ac33eb
--------------------------------------------------
✅ SUCCESS: Fetched block header from hash
Chain Tip Header:
Height  Hash                                                              Version   Prev-Hash                                                         Merkle-Root                                                       Time        Bits      Nonce
------  ----------------------------------------------------------------  --------  ----------------------------------------------------------------  ----------------------------------------------------------------  ----------  --------  --------
575045  000000000000000004a288072ebb35e37233f419918f9783d499979cb6ac33eb  2000e000  00000000000000000988156c7075dc9147a5b62922f1310862e8b9000d46dd9b  4ebcba09addd720991d03473f39dce4b9a72cc164e505cd446687a54df9b1585  1553416668  180997ee  87914848
============================================================
🎉 COMPLETED: Hash To Header

*/
