//go:build pkcs11

package pkcs11_test

import (
	"bytes"
	"crypto/sha256"
	"os"
	"strconv"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/hsm"
	"github.com/bsv-blockchain/go-wallet-toolbox/hsm/pkcs11"
)

// TestGrimselBRC42 requires a Securosys token and is skipped unless the
// operator exports the library, PIN file, and key label. SECUROSYS_PKCS11_CONF
// and SECUROSYS_SECRETS_CONF are read by the provider, not by this test.
// Failures report lengths only. Shared secrets and the PIN are not printed.
func TestGrimselBRC42(t *testing.T) {
	lib := os.Getenv("HSM_PKCS11_LIB")
	pinFile := os.Getenv("HSM_PIN_FILE")
	label := os.Getenv("HSM_KEY_LABEL")
	if lib == "" || pinFile == "" || label == "" {
		t.Skip("set HSM_PKCS11_LIB, HSM_PIN_FILE, and HSM_KEY_LABEL to run the HSM integration test")
	}
	slot := -1
	if raw := os.Getenv("HSM_SLOT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		require.NoError(t, err)
		slot = parsed
	}

	backend, err := pkcs11.Open(t.Context(), pkcs11.Config{
		LibraryPath:     lib,
		Slot:            slot,
		PINFile:         pinFile,
		KeyLabel:        label,
		SessionPoolSize: 2,
		Logf:            func(msg string) { t.Log(msg) },
	})
	require.NoError(t, err)

	deriver, err := hsm.New(t.Context(), backend, hsm.WithPublicKeyCacheSize(8))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, deriver.Close()) })

	owner := deriver.IdentityKey()
	require.NotNil(t, owner)
	counterparty, err := ec.NewPrivateKey()
	require.NoError(t, err)
	cpSDK := sdk.NewKeyDeriver(counterparty)
	ownerCP := sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: owner}
	hsmCP := sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: counterparty.PubKey()}
	protocol := sdk.Protocol{SecurityLevel: sdk.SecurityLevelEveryAppAndCounterparty, Protocol: "3241645161d8"}
	keyID := "hsm integration 1"

	t.Run("public key", func(t *testing.T) {
		forSelf, err := deriver.DerivePublicKey(t.Context(), protocol, keyID, hsmCP, true)
		require.NoError(t, err)
		wantSelf, err := cpSDK.DerivePublicKey(protocol, keyID, ownerCP, false)
		require.NoError(t, err)
		require.True(t, forSelf.IsEqual(wantSelf))

		forOther, err := deriver.DerivePublicKey(t.Context(), protocol, keyID, hsmCP, false)
		require.NoError(t, err)
		wantOther, err := cpSDK.DerivePublicKey(protocol, keyID, ownerCP, true)
		require.NoError(t, err)
		require.True(t, forOther.IsEqual(wantOther))
	})

	t.Run("sign", func(t *testing.T) {
		digest := sha256.Sum256([]byte("grimsel integration"))
		sig, err := deriver.Sign(t.Context(), protocol, keyID, hsmCP, digest[:])
		require.NoError(t, err)
		pub, err := cpSDK.DerivePublicKey(protocol, keyID, ownerCP, false)
		require.NoError(t, err)
		require.True(t, sig.Verify(digest[:], pub))
	})

	t.Run("symmetric key and hmac", func(t *testing.T) {
		got, err := deriver.DeriveSymmetricKey(t.Context(), protocol, keyID, hsmCP)
		require.NoError(t, err)
		want, err := cpSDK.DeriveSymmetricKey(protocol, keyID, ownerCP)
		require.NoError(t, err)
		requireEqualSecret(t, got.ToBytes(), want.ToBytes())

		payload := []byte("integration hmac")
		mac, err := deriver.CreateHMAC(t.Context(), protocol, keyID, hsmCP, payload)
		require.NoError(t, err)
		proto, err := sdk.NewProtoWallet(sdk.ProtoWalletArgs{Type: sdk.ProtoWalletArgsTypePrivateKey, PrivateKey: counterparty})
		require.NoError(t, err)
		wantMAC, err := proto.CreateHMAC(t.Context(), sdk.CreateHMACArgs{
			EncryptionArgs: sdk.EncryptionArgs{ProtocolID: protocol, KeyID: keyID, Counterparty: ownerCP},
			Data:           payload,
		}, "")
		require.NoError(t, err)
		requireEqualSecret(t, mac[:], wantMAC.HMAC[:])
	})

	t.Run("linkage", func(t *testing.T) {
		got, err := deriver.RevealSpecificSecret(t.Context(), hsmCP, protocol, keyID)
		require.NoError(t, err)
		want, err := cpSDK.RevealSpecificSecret(ownerCP, protocol, keyID)
		require.NoError(t, err)
		requireEqualSecret(t, got, want)

		point, err := deriver.RevealCounterpartySecret(t.Context(), hsmCP)
		require.NoError(t, err)
		wantPoint, err := cpSDK.RevealCounterpartySecret(ownerCP)
		require.NoError(t, err)
		require.True(t, point.IsEqual(wantPoint))
	})
}

func requireEqualSecret(t *testing.T, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Fatalf("secret mismatch (len got=%d want=%d)", len(got), len(want))
	}
}
