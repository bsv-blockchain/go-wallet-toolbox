package certifier

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

func TestCreateCertifierWalletDoesNotEchoInvalidPrivateKey(t *testing.T) {
	cfg := ConfigDefaults()
	cfg.CertifierWallet.PrivateKey = "not-a-private-key-secret"

	_, _, err := createCertifierWallet(t.Context(), &cfg, logging.NewTestLogger(t))

	require.Error(t, err)
	require.NotContains(t, err.Error(), cfg.CertifierWallet.PrivateKey)
	require.ErrorContains(t, err, "failed to parse certifier wallet private key")
}
