//go:build pkcs11

package infra

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/hsm"
	"github.com/bsv-blockchain/go-wallet-toolbox/hsm/pkcs11"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/keyderiver"
)

// Compile-time check that the HSM deriver satisfies the toolbox seam.
var _ keyderiver.Deriver = (*hsm.Deriver)(nil)

func openHSMDeriver(ctx context.Context, cfg HSMConfig, logger *slog.Logger) (keyderiver.Deriver, error) {
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
