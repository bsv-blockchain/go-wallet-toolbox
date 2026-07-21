package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Default window and backoff for waiting on infra at process start.
const (
	connectRetryWindow  = 30 * time.Second
	connectRetryInitial = 1 * time.Second
	connectRetryMax     = 5 * time.Second
)

// retryWithBackoff runs attempt until it returns nil, ctx is cancelled, or
// window elapses. Uses exponential backoff from initial up to maxBackoff.
// onRetry is optional and is called before each sleep after a failed attempt.
func retryWithBackoff(
	ctx context.Context,
	window, initial, maxBackoff time.Duration,
	attempt func() error,
	onRetry func(n int, err error, sleep time.Duration),
) error {
	deadline := time.Now().Add(window)
	backoff := initial
	var lastErr error

	for n := 1; ; n++ {
		err := attempt()
		if err == nil {
			return nil
		}
		lastErr = err

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("after %s (%d attempts): %w", window, n, lastErr)
		}

		sleep := backoff
		if sleep > remaining {
			sleep = remaining
		}
		if onRetry != nil {
			onRetry(n, err, sleep)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("aborted: %w", ctx.Err())
		case <-time.After(sleep):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// connectWallet builds a remote-storage wallet and probes first storage use.
// On infra unreachable it retries with exponential backoff for connectRetryWindow,
// then returns a clear error. ctx cancellation aborts early.
func connectWallet(
	ctx context.Context,
	network defs.BSVNetwork,
	priv *ec.PrivateKey,
	serverURL string,
	logger *slog.Logger,
) (*wallet.Wallet, error) {
	if logger == nil {
		logger = slog.Default()
	}

	var connected *wallet.Wallet
	err := retryWithBackoff(ctx, connectRetryWindow, connectRetryInitial, connectRetryMax,
		func() error {
			if connected != nil {
				connected.Close()
				connected = nil
			}
			w, err := wallet.NewWithStorageFactory(network, priv, func(userWallet sdk.Interface) (wdk.WalletStorageProvider, func(), error) {
				return storage.NewClient(serverURL, userWallet)
			})
			if err != nil {
				return err
			}
			// First real storage RPC (MakeAvailable path). NewClient itself does not dial.
			if _, err = w.Balance(ctx); err != nil {
				w.Close()
				return err
			}
			connected = w
			return nil
		},
		func(n int, err error, sleep time.Duration) {
			logger.Warn("storage not ready, retrying",
				"server_url", serverURL,
				"attempt", n,
				"error", err,
				"retry_in", sleep.String(),
			)
		},
	)
	if err != nil {
		if connected != nil {
			connected.Close()
		}
		return nil, fmt.Errorf("infra unreachable at %s: %w", serverURL, err)
	}
	return connected, nil
}
