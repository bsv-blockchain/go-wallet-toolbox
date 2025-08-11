package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"log/slog"
)

func main() {
	const (
		txID    = "323f6413e49b46fe58810b84f8aa912c53f6ef436b9e5dfcb9a78a6000efbb32"
		network = defs.NetworkMainnet
	)

	show.ProcessStart("Get BEEF by TxID")
	cfg := defs.DefaultServicesConfig(network)
	srv := services.New(slog.Default(), cfg)

	show.Step("Wallet-Services", fmt.Sprintf("fetching BEEF from services for txID: %q", txID))

	ctx := context.Background()
	beef, err := srv.GetBEEF(ctx, txID, nil)
	if err != nil {
		panic(fmt.Errorf("failed to fetch BEEF: %w", err))
	}

	show.Success(fmt.Sprintf("Success, found a BEEF that contains %d transactions and %d BUMPS", len(beef.Transactions), len(beef.BUMPs)))

	bytes, err := beef.Bytes()
	if err != nil {
		panic(fmt.Errorf("failed to serialize BEEF: %w", err))
	}

	beefHex := hex.EncodeToString(bytes)

	show.Beef(beefHex)
}

/* Output:

🚀 STARTING: Get BEEF by TxID
============================================================

=== STEP ===
Wallet-Services is performing: fetching BEEF from services for txID: "f164e822e38f456f94de9f2b5089276b62dc7365ee68eb06c2919f9f5dcc55e3"
--------------------------------------------------
2025/08/08 10:31:43 WARN error when calling service service=services.MerklePath service.name=ARC error="tx f164e822e38f456f94de9f2b5089276b62dc7365ee68eb06c2919f9f5dcc55e3 not found"
✅ SUCCESS: Success, found a BEEF that contains 1 transactions and 1 BUMPS

============================================================
BEEF HEX
============================================================
"0200beef01feccc019000802ba02e355cc5d9f9f91c206eb68ee6573dc626b2789502b9fde946f458fe322e864f1bb01015c0038a1edb4465400fe88a39655970d3948900cea36faf193fce092806051af7cfe012f00383b00b52ce49c70058eb805b7d8daba4b560aca4585bc8dbe6b3734a597d580011600e65e8fdca6b7ba6d98f59242678afee369cbb217240ea25ecabb7a4351d92f75010a00dc6a35c7167370801207f356964acd79ee1d640b07239626ebe7034f0123f5230104005f48e1e33bfda67455da2018e9ae1f2f7a8ee002e83b9286e5463e0010d7998d0103003c1e5e67fb51947498ddf1a132a2bdf3a707cea586f1b963ac9508265207e8df0100006bdfb81a46bb81f71df0cdf73238de58b9300e194462da33c37614f39bc8d57a0101000100000001ef8471be22a93b36dbedfaa6354c052ade0265a7a00897e3b0aff5bef66b5de4570000006b483045022100dcc5aaad606dd1cd81305f293b3d8a01214a76fd224bd1c8592fb3669ab2b10802200cd1236554250e23fe7f19d47a263671c3db5925bf640e9321a6ca89b8d6134741210292acdb57c788c1e8c83cdb0ae8f23e079139ba7ba1bccf67b31653c7af12c4b4ffffffff0140860100000000001976a914c0ffe0da73403a55ae0e0d7e90f42d9db607efd288ac00000000"

*/
