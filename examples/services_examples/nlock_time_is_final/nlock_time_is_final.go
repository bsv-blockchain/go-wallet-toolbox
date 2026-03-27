package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

// examplePastLockTime demonstrates how to check NLockTime finality for timestamp.
// Example #1: Timestamp locktime (past) - should be final
func examplePastLockTime(srv *services.WalletServices) {
	pastLockTime, err := to.UInt32(time.Now().Unix() - 3600)
	if err != nil {
		panic(fmt.Errorf("failed to compute past locktime: %w", err))
	}
	show.Step("Wallet-Services", fmt.Sprintf("Checking finality for past timestamp locktime: %d", pastLockTime))
	isFinal, err := srv.NLockTimeIsFinal(context.Background(), pastLockTime)
	if err != nil {
		show.WalletError("NLockTimeIsFinal", pastLockTime, err)
		panic(fmt.Errorf("failed to check nLockTime finality: %w", err))
	} else {
		show.WalletSuccess("NLockTimeIsFinal", pastLockTime, isFinal)
	}
}

// exampleFutureLockTime demonstrates how to check NLockTime finality for timestamp.
// Example #2: Timestamp locktime (future) - not final
func exampleFutureLockTime(srv *services.WalletServices) {
	futureLockTime, err := to.UInt32(time.Now().Unix() + 3600)
	if err != nil {
		panic(fmt.Errorf("failed to compute future locktime: %w", err))
	}
	show.Step("Wallet-Services", fmt.Sprintf("Checking finality for future timestamp locktime: %d", futureLockTime))
	isFinal, err := srv.NLockTimeIsFinal(context.Background(), futureLockTime)
	if err != nil {
		show.WalletError("NLockTimeIsFinal", futureLockTime, err)
		panic(fmt.Errorf("failed to check nLockTime finality: %w", err))
	} else {
		show.WalletSuccess("NLockTimeIsFinal", futureLockTime, isFinal)
	}
}

// exampleBlockHeightLockTime demonstrates how to check if a block height locktime is final.
// Example #3: Block height locktime - compared with chain height
// This will trigger a height lookup using the fallback chain (WoC -> Bitails -> BHS)
func exampleBlockHeightLockTime(srv *services.WalletServices) {
	blockHeightLockTime, err := to.UInt32(800_000)
	if err != nil {
		panic(fmt.Errorf("failed to compute block height locktime: %w", err))
	}
	show.Step("Wallet-Services", fmt.Sprintf("Checking finality for block height locktime: %d", blockHeightLockTime))
	isFinal, err := srv.NLockTimeIsFinal(context.Background(), blockHeightLockTime)
	if err != nil {
		show.WalletError("NLockTimeIsFinal", blockHeightLockTime, err)
		panic(fmt.Errorf("failed to check nLockTime finality: %w", err))
	} else {
		show.WalletSuccess("NLockTimeIsFinal", blockHeightLockTime, isFinal)
	}
}

func main() {
	show.ProcessStart("Check nLockTime Finality")

	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
	cfg.BHS.APIKey = "..." // use default api key DefaultAppToken from the BHS service https://github.com/bsv-blockchain/block-headers-service/blob/main/config/defaults.go#L8

	srv := services.New(slog.Default(), cfg)

	examplePastLockTime(srv)
	exampleFutureLockTime(srv)
	exampleBlockHeightLockTime(srv)

	show.ProcessComplete("Check nLockTime Finality")
}
