// Package connect builds a remote-storage operator wallet with startup retries.
package connect

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

const (
	connectRetryWindow  = 30 * time.Second
	connectRetryInitial = 1 * time.Second
	connectRetryMax     = 5 * time.Second
)

// Wallet connects to remote storage, probing with Balance until ready.
func Wallet(
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
	err := retryWithBackoff(
		ctx, connectRetryWindow, connectRetryInitial, connectRetryMax,
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
			if _, err = w.Balance(ctx); err != nil {
				w.Close()
				return err
			}
			connected = w
			return nil
		},
		func(n int, err error, sleep time.Duration) {
			logger.Warn(
				"storage not ready, retrying",
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
