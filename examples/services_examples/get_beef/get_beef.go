package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
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
