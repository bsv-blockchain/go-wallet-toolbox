//go:build !pkcs11

package infra

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/keyderiver"
)

func openHSMDeriver(_ context.Context, _ HSMConfig, _ *slog.Logger) (keyderiver.Deriver, error) {
	return nil, fmt.Errorf("hsm.enabled requires a binary built with -tags pkcs11")
}
