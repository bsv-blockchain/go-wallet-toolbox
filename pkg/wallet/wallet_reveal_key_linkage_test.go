package wallet_test

import (
	"context"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func TestRevealCounterpartyKeyLinkageOriginatorValidation(t *testing.T) {
	RunOriginatorValidationErrorTests(t,
		func(w *wallet.Wallet, ctx context.Context, originator string) (*sdk.RevealCounterpartyKeyLinkageResult, error) {
			args := sdk.RevealCounterpartyKeyLinkageArgs{
				Counterparty: testusers.Bob.PublicKey(t),
				Verifier:     testusers.Alice.PublicKey(t),
			}
			return w.RevealCounterpartyKeyLinkage(ctx, args, originator)
		},
	)
}

func TestWallet_RevealCounterpartyKeyLinkage(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	aliceWallet := given.Wallet().WithSQLiteStorage().WithServices().ForUser(testusers.Alice)

	verifierKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	// when
	result, err := aliceWallet.RevealCounterpartyKeyLinkage(t.Context(), sdk.RevealCounterpartyKeyLinkageArgs{
		Counterparty: testusers.Bob.PublicKey(t),
		Verifier:     verifierKey.PubKey(),
	}, fixtures.DefaultOriginator)

	// then
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.EncryptedLinkage)
	assert.NotEmpty(t, result.EncryptedLinkageProof)
	require.NotNil(t, result.Prover)
	assert.Equal(t, testusers.Alice.PublicKey(t), result.Prover)
	assert.Equal(t, testusers.Bob.PublicKey(t), result.Counterparty)
	assert.Equal(t, verifierKey.PubKey(), result.Verifier)

	// and: the verifier can decrypt the linkage and proof
	verifierProto, err := sdk.NewProtoWallet(sdk.ProtoWalletArgs{Type: sdk.ProtoWalletArgsTypePrivateKey, PrivateKey: verifierKey})
	require.NoError(t, err)

	decryptedLinkage, err := verifierProto.Decrypt(t.Context(), sdk.DecryptArgs{
		Ciphertext: result.EncryptedLinkage,
		EncryptionArgs: sdk.EncryptionArgs{
			ProtocolID:   sdk.Protocol{SecurityLevel: 2, Protocol: "counterparty linkage revelation"},
			KeyID:        result.RevelationTime,
			Counterparty: sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: result.Prover},
		},
	}, fixtures.DefaultOriginator)
	require.NoError(t, err)

	// and: compute expected linkage - Alice private with Bob public
	expectedSharedSecret, err := testusers.Alice.PrivateKey(t).DeriveSharedSecret(testusers.Bob.PublicKey(t))
	require.NoError(t, err)
	assert.Equal(t, expectedSharedSecret.Compressed(), []byte(decryptedLinkage.Plaintext))

	decryptedProof, err := verifierProto.Decrypt(t.Context(), sdk.DecryptArgs{
		Ciphertext: result.EncryptedLinkageProof,
		EncryptionArgs: sdk.EncryptionArgs{
			ProtocolID:   sdk.Protocol{SecurityLevel: 2, Protocol: "counterparty linkage revelation"},
			KeyID:        result.RevelationTime,
			Counterparty: sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: result.Prover},
		},
	}, fixtures.DefaultOriginator)
	require.NoError(t, err)
	assert.Len(t, decryptedProof.Plaintext, 98)
}

func TestRevealSpecificKeyLinkageOriginatorValidation(t *testing.T) {
	RunOriginatorValidationErrorTests(t,
		func(w *wallet.Wallet, ctx context.Context, originator string) (*sdk.RevealSpecificKeyLinkageResult, error) {
			args := sdk.RevealSpecificKeyLinkageArgs{
				Counterparty: sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: testusers.Bob.PublicKey(t)},
				Verifier:     testusers.Alice.PublicKey(t),
				ProtocolID:   sdk.Protocol{SecurityLevel: 0, Protocol: "tests"},
				KeyID:        "test key id",
			}
			return w.RevealSpecificKeyLinkage(ctx, args, originator)
		},
	)
}

func TestWallet_RevealSpecificKeyLinkage(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	aliceWallet := given.Wallet().WithSQLiteStorage().WithServices().ForUser(testusers.Alice)

	verifierKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	protocolID := sdk.Protocol{SecurityLevel: 0, Protocol: "tests"}
	keyID := "test key id"

	// when:
	result, err := aliceWallet.RevealSpecificKeyLinkage(t.Context(), sdk.RevealSpecificKeyLinkageArgs{
		Counterparty: sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: testusers.Bob.PublicKey(t)},
		Verifier:     verifierKey.PubKey(),
		ProtocolID:   protocolID,
		KeyID:        keyID,
	}, fixtures.DefaultOriginator)

	// then:
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.EncryptedLinkage)
	assert.NotEmpty(t, result.EncryptedLinkageProof)
	assert.Equal(t, testusers.Alice.PublicKey(t), result.Prover)
	assert.Equal(t, testusers.Bob.PublicKey(t), result.Counterparty)
	assert.Equal(t, verifierKey.PubKey(), result.Verifier)
	assert.Equal(t, protocolID, result.ProtocolID)
	assert.Equal(t, keyID, result.KeyID)
	assert.Equal(t, byte(0), result.ProofType)

	verifierProto, err := sdk.NewProtoWallet(sdk.ProtoWalletArgs{Type: sdk.ProtoWalletArgsTypePrivateKey, PrivateKey: verifierKey})
	require.NoError(t, err)

	decryptedLinkage, err := verifierProto.Decrypt(t.Context(), sdk.DecryptArgs{
		Ciphertext: result.EncryptedLinkage,
		EncryptionArgs: sdk.EncryptionArgs{
			ProtocolID:   sdk.Protocol{SecurityLevel: 2, Protocol: "specific linkage revelation 0 tests"},
			KeyID:        keyID,
			Counterparty: sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: result.Prover},
		},
	}, fixtures.DefaultOriginator)
	require.NoError(t, err)

	// and: expected linkage using Alice's key deriver
	kd := sdk.NewKeyDeriver(testusers.Alice.PrivateKey(t))
	expectedLinkage, err := kd.RevealSpecificSecret(
		sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: testusers.Bob.PublicKey(t)},
		protocolID,
		keyID,
	)
	require.NoError(t, err)
	assert.Equal(t, expectedLinkage, []byte(decryptedLinkage.Plaintext))

	decryptedProof, err := verifierProto.Decrypt(t.Context(), sdk.DecryptArgs{
		Ciphertext: result.EncryptedLinkageProof,
		EncryptionArgs: sdk.EncryptionArgs{
			ProtocolID:   sdk.Protocol{SecurityLevel: 2, Protocol: "specific linkage revelation 0 tests"},
			KeyID:        keyID,
			Counterparty: sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: result.Prover},
		},
	}, fixtures.DefaultOriginator)
	require.NoError(t, err)
	assert.Equal(t, []byte{0}, []byte(decryptedProof.Plaintext))
}
