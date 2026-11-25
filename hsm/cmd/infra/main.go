//go:build pkcs11

// Command infra is the storage server with the Securosys PKCS#11 root key.
// Build it with -tags pkcs11. The toolbox's cmd/infra does not link this module.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bsv-blockchain/go-wallet-toolbox/hsm"
	"github.com/bsv-blockchain/go-wallet-toolbox/hsm/pkcs11"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/infra"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/keyderiver"
)

func main() {
	server, err := infra.NewServer(
		context.Background(),
		infra.WithConfigFile("infra-config.yaml"),
		infra.WithKeyDeriverFactory(openSecurosys),
	)
	if err != nil {
		panic(err)
	}

	go func() {
		if err = server.ListenAndServe(context.Background()); err != nil {
			panic(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	server.Cleanup()
}

func openSecurosys(ctx context.Context, cfg infra.HSMConfig, logger *slog.Logger) (keyderiver.Deriver, error) {
	backend, err := pkcs11.Open(ctx, pkcs11.Config{
		LibraryPath:     cfg.LibraryPath,
		Slot:            cfg.Slot,
		PINFile:         cfg.PINFile,
		KeyLabel:        cfg.KeyLabel,
		SessionPoolSize: cfg.SessionPoolSize,
		Logf: func(msg string) {
			if logger != nil {
				logger.Info(msg)
			}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("open pkcs11 backend: %w", err)
	}
	deriver, err := hsm.New(ctx, backend, hsm.WithPublicKeyCacheSize(cfg.PublicKeyCacheSize))
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("create hsm key deriver: %w", err)
	}
	return deriver, nil
}
