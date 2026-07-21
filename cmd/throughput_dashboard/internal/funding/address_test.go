package funding_test

import (
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/funding"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/stretchr/testify/require"
)

func TestDeriveInfoMainnet(t *testing.T) {
	priv, err := ec.NewPrivateKey()
	require.NoError(t, err)

	info, err := funding.DeriveInfo(priv, defs.NetworkMainnet, 50_000)
	require.NoError(t, err)
	require.Equal(t, "main", info.Network)
	require.NotEmpty(t, info.Address)
	require.NotEmpty(t, info.LockingScriptHex)
	require.Equal(t, funding.DerivationPrefixB64, info.DerivationPrefixB64)
	require.Equal(t, uint64(50_000), info.SuggestedSatoshis)
	require.Equal(t, priv.PubKey().ToDERHex(), info.OperatorIdentityHex)
}

func TestAnyonePaymentRemittance(t *testing.T) {
	p, err := funding.AnyonePaymentRemittance()
	require.NoError(t, err)
	require.NotEmpty(t, p.DerivationPrefix)
	require.NotEmpty(t, p.DerivationSuffix)
	require.NotNil(t, p.SenderIdentityKey)
}
