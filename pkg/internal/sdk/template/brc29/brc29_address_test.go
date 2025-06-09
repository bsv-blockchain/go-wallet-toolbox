package brc29_test

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/sdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/sdk/template/brc29"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddress(t *testing.T) {
	errorTestCases := map[string]struct {
		sender    string
		keyID     brc29.KeyID
		recipient string
	}{
		"return error when sender key is empty": {
			sender:    "",
			keyID:     keyID,
			recipient: invalidKeyHex,
		},
		"return error when sender key parsing fails": {
			sender:    invalidKeyHex,
			keyID:     keyID,
			recipient: recipientPublicKeyHex,
		},
		"return error when KeyID is invalid": {
			sender:    senderPrivateKeyHex,
			keyID:     brc29.KeyID{DerivationPrefix: "", DerivationSuffix: ""},
			recipient: recipientPublicKeyHex,
		},
		"return error when recipient key is empty": {
			sender:    senderPrivateKeyHex,
			keyID:     keyID,
			recipient: "",
		},
		"return error when recipient key parsing fails": {
			sender:    senderPrivateKeyHex,
			keyID:     keyID,
			recipient: invalidKeyHex,
		},
	}
	for name, test := range errorTestCases {
		t.Run(name, func(t *testing.T) {
			address, err := brc29.Address(test.sender, test.keyID, test.recipient)

			require.Nil(t, address)
			require.Error(t, err)
		})
	}

	t.Run("return error when nil is passed as sender private key deriver", func(t *testing.T) {
		var keyDeriver *sdk.KeyDeriver

		address, err := brc29.Address(keyDeriver, keyID, recipientPublicKeyHex)

		assert.Error(t, err)
		require.Nil(t, address)
	})

	t.Run("return error when nil is passed as sender private key", func(t *testing.T) {
		var priv *ec.PrivateKey

		address, err := brc29.Address(priv, keyID, recipientPublicKeyHex)

		assert.Error(t, err)
		require.Nil(t, address)
	})

	t.Run("return error when nil is passed as recipient public key deriver", func(t *testing.T) {
		var keyDeriver *sdk.KeyDeriver

		address, err := brc29.Address(senderPrivateKeyHex, keyID, keyDeriver)

		assert.Error(t, err)
		require.Nil(t, address)
	})

	t.Run("return error when nil is passed as recipient public key", func(t *testing.T) {
		var pub *ec.PublicKey

		address, err := brc29.Address(senderPrivateKeyHex, keyID, pub)

		assert.Error(t, err)
		require.Nil(t, address)
	})

	t.Run("return valid address created with brc28 with hex string as sender private key source", func(t *testing.T) {
		address, err := brc29.Address(senderPrivateKeyHex, keyID, recipientPublicKeyHex)

		assert.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedAddress, address.AddressString)
	})

	t.Run("return valid address created with brc28 with wif as sender private key source", func(t *testing.T) {
		address, err := brc29.Address(brc29.WIF(senderWIFString), keyID, recipientPublicKeyHex)

		assert.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedAddress, address.AddressString)
	})

	t.Run("return valid address created with brc28 with ec.PrivateKey as sender private key source", func(t *testing.T) {
		priv, err := ec.PrivateKeyFromHex(senderPrivateKeyHex)
		require.NoError(t, err)

		address, err := brc29.Address(priv, keyID, recipientPublicKeyHex)

		assert.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedAddress, address.AddressString)
	})

	t.Run("return valid address created with brc28 with key deriver as sender private key source", func(t *testing.T) {
		priv, err := ec.PrivateKeyFromHex(senderPrivateKeyHex)
		require.NoError(t, err)

		keyDeriver := sdk.NewKeyDeriver(priv)

		address, err := brc29.Address(keyDeriver, keyID, recipientPublicKeyHex)

		assert.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedAddress, address.AddressString)
	})

	t.Run("return valid address created with brc28 with ec.PublicKey as receiver public key source", func(t *testing.T) {
		pub, err := ec.PublicKeyFromString(recipientPublicKeyHex)
		require.NoError(t, err)

		address, err := brc29.Address(senderPrivateKeyHex, keyID, pub)

		assert.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedAddress, address.AddressString)
	})

	t.Run("return testnet address created with brc29", func(t *testing.T) {
		pub, err := ec.PublicKeyFromString(recipientPublicKeyHex)
		require.NoError(t, err)

		address, err := brc29.Address(senderPrivateKeyHex, keyID, pub, brc29.WithTestNet())

		assert.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedTestnetAddress, address.AddressString)
	})

}
