//go:build cgo && !nobdk && !ios && !android && (darwin || linux) && (amd64 || arm64)

package signaturebackend_test

import (
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction/bdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/signaturebackend"
)

func TestCgoBuild_UsesGoBDK(t *testing.T) {
	assert.Equal(t, "gobdk-libsecp256k1", signaturebackend.Name)
}

// GoBDK signs as well as verifies. Its signatures must verify under go-sdk's
// built-in verifier, or transactions signed here would be rejected by peers
// that do not use GoBDK.
func TestCgoBuild_GoBDKSignaturesVerifyUnderBuiltInVerifier(t *testing.T) {
	key, err := ec.NewPrivateKey()
	require.NoError(t, err)
	tx := p2pkhSpend(t, key)

	bdk.ResetSignatureBackend()
	t.Cleanup(bdk.InstallSignatureBackend)

	assert.NoError(t, verify(tx))
}
