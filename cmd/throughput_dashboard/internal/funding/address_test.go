package funding_test

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/funding"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/stretchr/testify/require"
)

// Fixed operator key for deterministic address regression checks.
const operatorPrivHex = "0000000000000000000000000000000000000000000000000000000000000001"

func testOperatorPriv(t *testing.T) *ec.PrivateKey {
	t.Helper()
	priv, err := ec.PrivateKeyFromHex(operatorPrivHex)
	require.NoError(t, err)
	return priv
}

func TestDeriveInfoMainnet(t *testing.T) {
	priv := testOperatorPriv(t)

	info, err := funding.DeriveInfo(priv, defs.NetworkMainnet, 50_000)
	require.NoError(t, err)
	require.Equal(t, "main", info.Network)
	require.NotEmpty(t, info.Address)
	require.NotEmpty(t, info.LockingScriptHex)
	// Locking script must be valid hex for WalletClient.
	_, err = hex.DecodeString(info.LockingScriptHex)
	require.NoError(t, err)
	require.Equal(t, funding.DerivationPrefixB64, info.DerivationPrefixB64)
	require.Equal(t, funding.DerivationSuffixB64, info.DerivationSuffixB64)
	require.Equal(t, uint64(50_000), info.SuggestedSatoshis)
	require.Equal(t, priv.PubKey().ToDERHex(), info.OperatorIdentityHex)

	_, anyonePub := sdk.AnyoneKey()
	require.Equal(t, anyonePub.ToDERHex(), info.SenderIdentityKeyHex)

	// Cross-check against brc29.AddressForSelf with the same inputs.
	keyID := brc29.KeyID{
		DerivationPrefix: funding.DerivationPrefixB64,
		DerivationSuffix: funding.DerivationSuffixB64,
	}
	expected, err := brc29.AddressForSelf(anyonePub, keyID, priv, brc29.WithMainNet())
	require.NoError(t, err)
	require.Equal(t, expected.AddressString, info.Address)
}

func TestDeriveInfoTestnet(t *testing.T) {
	priv := testOperatorPriv(t)

	info, err := funding.DeriveInfo(priv, defs.NetworkTestnet, 1_000)
	require.NoError(t, err)
	require.Equal(t, "test", info.Network)
	require.NotEmpty(t, info.Address)
	require.NotEmpty(t, info.LockingScriptHex)
	require.Equal(t, uint64(1_000), info.SuggestedSatoshis)

	_, anyonePub := sdk.AnyoneKey()
	keyID := brc29.KeyID{
		DerivationPrefix: funding.DerivationPrefixB64,
		DerivationSuffix: funding.DerivationSuffixB64,
	}
	expected, err := brc29.AddressForSelf(anyonePub, keyID, priv, brc29.WithTestNet())
	require.NoError(t, err)
	require.Equal(t, expected.AddressString, info.Address)

	// Mainnet and testnet addresses must differ for the same key.
	main, err := funding.DeriveInfo(priv, defs.NetworkMainnet, 1_000)
	require.NoError(t, err)
	require.NotEqual(t, main.Address, info.Address)
}

func TestDeriveInfoDefaultSuggested(t *testing.T) {
	priv := testOperatorPriv(t)
	info, err := funding.DeriveInfo(priv, defs.NetworkMainnet, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(100_000), info.SuggestedSatoshis)
}

func TestDeriveInfoNilPriv(t *testing.T) {
	_, err := funding.DeriveInfo(nil, defs.NetworkMainnet, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "private key")
}

func TestDeriveInfoInvalidNetwork(t *testing.T) {
	priv := testOperatorPriv(t)
	_, err := funding.DeriveInfo(priv, defs.BSVNetwork("not-a-network"), 0)
	require.Error(t, err)
}

func TestAnyonePaymentRemittance(t *testing.T) {
	p, err := funding.AnyonePaymentRemittance()
	require.NoError(t, err)
	require.NotNil(t, p)

	wantPrefix, err := base64.StdEncoding.DecodeString(funding.DerivationPrefixB64)
	require.NoError(t, err)
	wantSuffix, err := base64.StdEncoding.DecodeString(funding.DerivationSuffixB64)
	require.NoError(t, err)
	require.Equal(t, wantPrefix, p.DerivationPrefix)
	require.Equal(t, wantSuffix, p.DerivationSuffix)

	_, anyonePub := sdk.AnyoneKey()
	require.NotNil(t, p.SenderIdentityKey)
	require.Equal(t, anyonePub.ToDERHex(), p.SenderIdentityKey.ToDERHex())
}

func TestDerivationConstantsMatchFaucet(t *testing.T) {
	// Keep faucet / loadgen / dashboard derivation aligned.
	require.Equal(t, "SfKxPIJNgdI=", funding.DerivationPrefixB64)
	require.Equal(t, "NaGLC6fMH50=", funding.DerivationSuffixB64)
}
