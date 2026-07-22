// Command throughput_dashboard runs a mainnet-oriented demo control plane for
// wallet-infra throughput mode: FuelKeeper, start/stop OP_RETURN createAction
// stream, and a localhost web UI (TPS, fuel balances, top-ups, WalletClient funding).
package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"

	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/api"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/connect"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/metrics"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/stream"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/syncwallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/fuelkeeper"
)

//go:embed web/*
var webRoot embed.FS

func main() {
	logger := slog.Default()
	if err := run(logger); err != nil {
		logger.Error("throughput dashboard failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.ConfigFromEnv()
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

	logger.Info("connecting to storage",
		"server_url", cfg.ServerURL,
		"network", network,
		"http_addr", cfg.HTTPAddr,
	)

	operatorWallet, err := connect.Wallet(ctx, network, priv, cfg.ServerURL, logger)
	if err != nil {
		return fmt.Errorf("create wallet: %w", err)
	}
	// Serialize all storage RPCs: concurrent AuthFetch handshakes deadlock and
	// leave FuelKeeper + sampler stuck so the UI never shows internalized funds.
	wallet := syncwallet.New(operatorWallet)
	defer wallet.Close()

	throughput := config.DemoThroughput()
	denom, err := throughput.Denomination(defs.DefaultFeeModel(), defs.DefaultCommission())
	if err != nil {
		return fmt.Errorf("resolve denomination: %w", err)
	}
	targetPool := throughput.TargetPool()
	logger.Info("throughput profile",
		"denomination_satoshis", denom,
		"target_tps", throughput.TargetTPS,
		"target_pool", targetPool,
		"low_water_percent", throughput.LowWaterPercent,
		"high_water_percent", throughput.HighWaterPercent,
	)

	keeper, err := fuelkeeper.New(wallet, fuelkeeper.FromThroughput(throughput, denom), logger)
	if err != nil {
		return fmt.Errorf("create fuel keeper: %w", err)
	}
	go keeper.Run(ctx)

	ctrl := stream.NewController(wallet, stream.Options{
		TPS:        cfg.TPS,
		Workers:    cfg.Workers,
		Originator: cfg.Originator,
	}, logger)

	sampler := metrics.NewSampler(
		wallet,
		ctrl,
		cfg.Originator,
		time.Duration(cfg.SampleSeconds)*time.Second,
		throughput.TargetTPS,
		denom,
		targetPool,
		throughput.LowWaterPercent,
		throughput.HighWaterPercent,
		logger,
	)
	go sampler.Run(ctx)

	webFS, err := fs.Sub(webRoot, "web")
	if err != nil {
		return fmt.Errorf("embed web fs: %w", err)
	}

	srv := api.New(api.Deps{
		Ctrl:       ctrl,
		Sampler:    sampler,
		Wallet:     wallet,
		Priv:       priv,
		Network:    network,
		Originator: cfg.Originator,
		ServerURL:  cfg.ServerURL,
		Logger:     logger,
		WebFS:      webFS,
		ParentCtx:  ctx,
	})

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("dashboard listening", "addr", cfg.HTTPAddr, "network", network)
		if listenErr := httpServer.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			errCh <- listenErr
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case listenErr := <-errCh:
		if listenErr != nil {
			return listenErr
		}
	}

	ctrl.Stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	return nil
}
