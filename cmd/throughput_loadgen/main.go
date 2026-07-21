package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/fuelkeeper"
)

// loadgenThroughput matches infra-config-docker-throughput.yaml profile fields
// that size the fuel pool / denomination for OP_RETURN createActions.
func loadgenThroughput() defs.Throughput {
	base := defs.DefaultUTXOManagement().Throughput
	base.ExpectedTxSizeBytes = 200
	base.ExpectedOutputSatoshis = 0
	base.TargetTPS = 1000
	return base
}

func main() {
	logger := slog.Default()
	if err := run(logger); err != nil {
		logger.Error("loadgen failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := ConfigFromEnv()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	priv, err := ec.PrivateKeyFromHex(cfg.PrivateKeyHex)
	if err != nil {
		return fmt.Errorf("parse PRIVATE_KEY: %w", err)
	}

	network, err := defs.ParseBSVNetworkStr(cfg.Network)
	if err != nil {
		return fmt.Errorf("parse BSV_NETWORK: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	operatorWallet, err := connectWallet(ctx, network, priv, cfg.ServerURL, logger)
	if err != nil {
		return fmt.Errorf("create wallet: %w", err)
	}
	defer operatorWallet.Close()

	if cfg.FaucetTxID != "" {
		if faucetErr := BootstrapFaucet(ctx, operatorWallet, network, cfg.FaucetTxID, cfg.Originator, logger); faucetErr != nil {
			return fmt.Errorf("faucet bootstrap: %w", faucetErr)
		}
	}

	throughput := loadgenThroughput()
	denom, err := throughput.Denomination(defs.DefaultFeeModel(), defs.DefaultCommission())
	if err != nil {
		return fmt.Errorf("resolve denomination: %w", err)
	}
	logger.Info("fuel denomination resolved", "denomination_satoshis", denom, "target_tps", throughput.TargetTPS)

	keeper, err := fuelkeeper.New(operatorWallet, fuelkeeper.FromThroughput(throughput, denom), logger)
	if err != nil {
		return fmt.Errorf("create fuel keeper: %w", err)
	}
	go keeper.Run(ctx)

	if cfg.WarmupSeconds > 0 {
		logger.Info("warming up", "seconds", cfg.WarmupSeconds)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(cfg.WarmupSeconds) * time.Second):
		}
	}

	locking, err := OpReturnLockingScript(ProofPayload)
	if err != nil {
		return fmt.Errorf("build opreturn locking script: %w", err)
	}

	logger.Info("starting load",
		"tps", cfg.TPS,
		"workers", cfg.Workers,
		"duration_seconds", cfg.DurationSeconds,
		"originator", cfg.Originator,
		"server_url", cfg.ServerURL,
	)
	stats := RunLoad(ctx, operatorWallet, cfg, locking)
	logger.Info("load complete",
		"attempted", stats.Attempted,
		"succeeded", stats.Succeeded,
		"failed", stats.Failed,
	)
	return nil
}
