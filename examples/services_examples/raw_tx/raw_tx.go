package main

import (
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

func main() {
	show.ProcessStart("Raw Transaction from ARC")

	txID := "9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6"
	network := defs.NetworkMainnet

	cfg := defs.DefaultServicesConfig(network)
	srv := services.New(slog.Default(), cfg)

	show.Step("Wallet-Services", fmt.Sprintf("fetching RawTx for txID %s using ARC", txID))
	rawTx, err := srv.RawTx(txID)
	if err != nil {
		panic(fmt.Errorf("failed to fetch raw transaction: %w", err))
	}

	show.Success("Fetched Raw Transaction")
	show.RawTxOutput(&rawTx)
	show.ProcessComplete("Raw Transaction from ARC")
}

/* Output:

🚀 STARTING: Raw Transaction from ARC
============================================================

=== STEP ===
Wallet-Services is performing: fetching RawTx for txID 9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6 using ARC
--------------------------------------------------
✅ SUCCESS: Fetched Raw Transaction

============================================================
RAW TRANSACTION RESULT
============================================================
Service: WhatsOnChain
TxID:   9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6
RawTx:  01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff170399c80d2f43555656452f0150cbfa27d51703e1a32500ffffffff01f3d4a112000000001976a914d648686cf603c11850f39600e37312738accca8f88ac00000000
============================================================
🎉 COMPLETED: Raw Transaction from ARC

*/
