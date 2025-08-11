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
	show.ProcessStart("Get Status For TxIDs")

	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
	srv := services.New(slog.Default(), cfg)

	txIDs := []string{
		"6d12d017d610344453f0cf817ce355b6f4be40e3bc723bca8c6c92991cbbce70",
		"ab0f76f957662335f98ee430a665f924c28310ec5126c2aede56086f9233326f",
		"866c535d32935d97e021c06e81d824418cf257af1604fcaeb153afb5fe82cbe2",
		"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	}

	show.Step("Wallet-Services", "query depth/status for multiple txids")

	res, err := srv.GetStatusForTxIDs(context.Background(), txIDs)
	if err != nil {
		show.WalletError("GetStatusForTxIDs", map[string]any{"txids": txIDs}, err)
		panic(fmt.Errorf("failed to get status for txids: %w", err))
	}

	show.WalletSuccess("GetStatusForTxIDs", map[string]any{
		"txids": txIDs,
	}, "OK")
	show.GetStatusForTxIDsOutput(res)

	show.ProcessComplete("Get Status For TxIDs")
}

/* Output:

🚀 STARTING: Get Status For TxIDs
============================================================

=== STEP ===
Wallet-Services is performing: query depth/status for multiple txids
--------------------------------------------------

 WALLET CALL: GetStatusForTxids
Args: map[txids:[6d12d017d610344453f0cf817ce355b6f4be40e3bc723bca8c6c92991cbbce70 ab0f76f957662335f98ee430a665f924c28310ec5126c2aede56086f9233326f 866c535d32935d97e021c06e81d824418cf257af1604fcaeb153afb5fe82cbe2 ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff]]
✅ Result: OK

============================================================
TX STATUS (MULTI)
============================================================
Service: WhatsOnChain
Overall: success

Per-TX status:
TxID                                                              Status   Depth
----------------------------------------------------------------  -------  -----
6d12d017d610344453f0cf817ce355b6f4be40e3bc723bca8c6c92991cbbce70  mined    19
ab0f76f957662335f98ee430a665f924c28310ec5126c2aede56086f9233326f  mined    47848
866c535d32935d97e021c06e81d824418cf257af1604fcaeb153afb5fe82cbe2  mined    1
ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff  unknown  -
============================================================
🎉 COMPLETED: Get Status For TxIDs

*/
