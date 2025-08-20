package main

import (
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

func main() {
	const (
		scriptHex = "76a91489abcdefabbaabbaabbaabbaabbaabbaabbaabba88ac"
	)

	show.ProcessStart("Hash Output Script")
	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
	srv := services.New(slog.Default(), cfg)

	show.Step("Wallet-Services", fmt.Sprintf("hashing script hex: %q", scriptHex))

	hashed, err := srv.HashOutputScript(scriptHex)
	if err != nil {
		show.WalletError("HashOutputScript", scriptHex, err)
		panic(fmt.Errorf("failed to hash output script: %w", err))
	}

	show.WalletSuccess("HashOutputScript", scriptHex, hashed)
	show.ProcessComplete("Hash Output Script")
}

/* Output:

🚀 STARTING: Hash Output Script
============================================================

=== STEP ===
Wallet-Services is performing: hashing script hex: "76a91489abcdefabbaabbaabbaabbaabbaabbaabbaabba88ac"
--------------------------------------------------

 WALLET CALL: HashOutputScript
Args: 76a91489abcdefabbaabbaabbaabbaabbaabbaabbaabba88ac
✅ Result: db46d31e84e16e7fb031b3ab375131a7bb65775c0818dc17fe0d4444efb3d0aa
============================================================
🎉 COMPLETED: Hash Output Script

*/
