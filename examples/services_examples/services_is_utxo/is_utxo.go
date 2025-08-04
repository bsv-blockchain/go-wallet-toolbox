package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

func main() {
	const (
		scriptHash = "b3005d46af31c4b5675b73c17579b7bd366dfe10635b7b43ac111aea5226efb6"
		txidHex    = "ab0f76f957662335f98ee430a665f924c28310ec5126c2aede56086f9233326f"
		txIndex    = uint32(1)
	)

	show.ProcessStart("Is UTXO")

	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
	srv := services.New(slog.Default(), cfg)

	show.Step("Wallet-Services", "checking if outpoint is a UTXO")

	txid, err := chainhash.NewHashFromHex(txidHex)
	if err != nil {
		panic(fmt.Errorf("invalid txid: %w", err))
	}

	outpoint := &transaction.Outpoint{
		Txid:  *txid,
		Index: txIndex,
	}

	isUtxo, err := srv.IsUtxo(context.Background(), scriptHash, outpoint)
	if err != nil {
		show.WalletError("IsUtxo", map[string]any{
			"scriptHash": scriptHash,
			"txid":       txidHex,
			"index":      txIndex,
		}, err)
		return
	}

	show.WalletSuccess("IsUtxo", map[string]any{
		"scriptHash": scriptHash,
		"txid":       txidHex,
		"index":      txIndex,
	}, isUtxo)

	show.ProcessComplete("Is UTXO")
}

/* Output:

🚀 STARTING: Is UTXO
============================================================

=== STEP ===
Wallet-Services is performing: checking if outpoint is a UTXO
--------------------------------------------------

WALLET CALL: IsUtxo
Args: map[index:1 scriptHash:b3005d46af31c4b5675b73c17579b7bd366dfe10635b7b43ac111aea5226efb6 txid:ab0f76f957662335f98ee430a665f924c28310ec5126c2aede56086f9233326f]
✅ Result: true
============================================================
🎉 COMPLETED: Is UTXO

*/
