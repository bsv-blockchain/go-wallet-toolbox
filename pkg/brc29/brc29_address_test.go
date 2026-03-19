package brc29_test

import (
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
)

func TestBRC29AddressByRecipientCreation(t *testing.T) {
	t.Run("return valid address with hex string as sender public key source", func(t *testing.T) {
		address, err := brc29.AddressForSelf(brc29.PubHex(senderPublicKeyHex), keyID, brc29.PrivHex(recipientPrivateKeyHex))
		require.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedAddress, address.AddressString)
	})

	t.Run("return valid address with ec.PublicKey as sender public key source", func(t *testing.T) {
		pub, err := ec.PublicKeyFromString(senderPublicKeyHex)
		require.NoError(t, err)
		address, err := brc29.AddressForSelf(pub, keyID, brc29.PrivHex(recipientPrivateKeyHex))
		require.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedAddress, address.AddressString)
	})

	t.Run("return valid address with sender key deriver as sender public key source", func(t *testing.T) {
		priv, err := ec.PrivateKeyFromHex(senderPrivateKeyHex)
		require.NoError(t, err)
		keyDeriver := sdk.NewKeyDeriver(priv)
		address, err := brc29.AddressForSelf(keyDeriver, keyID, brc29.PrivHex(recipientPrivateKeyHex))
		require.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedAddress, address.AddressString)
	})

	t.Run("return valid address with ec.PrivateKey as recipient private key source", func(t *testing.T) {
		priv, err := ec.PrivateKeyFromHex(recipientPrivateKeyHex)
		require.NoError(t, err)
		address, err := brc29.AddressForSelf(brc29.PubHex(senderPublicKeyHex), keyID, priv)
		require.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedAddress, address.AddressString)
	})

	t.Run("return testnet address created with brc29 by recipient", func(t *testing.T) {
		address, err := brc29.AddressForSelf(brc29.PubHex(senderPublicKeyHex), keyID, brc29.PrivHex(recipientPrivateKeyHex), brc29.WithTestNet())
		require.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedTestnetAddress, address.AddressString)
	})
}

func TestBRC29AddressByRecipientErrors(t *testing.T) {
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
			recipient: recipientPrivateKeyHex,
		},
		"return error when KeyID is invalid": {
			sender:    senderPublicKeyHex,
			keyID:     brc29.KeyID{DerivationPrefix: "", DerivationSuffix: ""},
			recipient: recipientPrivateKeyHex,
		},
		"return error when recipient key is empty": {
			sender:    senderPublicKeyHex,
			keyID:     keyID,
			recipient: "",
		},
		"return error when recipient key parsing fails": {
			sender:    senderPublicKeyHex,
			keyID:     keyID,
			recipient: invalidKeyHex,
		},
	}
	for name, test := range errorTestCases {
		t.Run(name, func(t *testing.T) {
			address, err := brc29.AddressForSelf(brc29.PubHex(test.sender), test.keyID, brc29.PrivHex(test.recipient))
			require.Nil(t, address)
			require.Error(t, err)
		})
	}

	t.Run("return error when nil is passed as sender public key deriver", func(t *testing.T) {
		var keyDeriver *sdk.KeyDeriver
		address, err := brc29.AddressForSelf(keyDeriver, keyID, brc29.PrivHex(recipientPrivateKeyHex))
		require.Error(t, err)
		require.Nil(t, address)
	})

	t.Run("return error when nil is passed as sender public key", func(t *testing.T) {
		var pub *ec.PublicKey
		address, err := brc29.AddressForSelf(pub, keyID, brc29.PrivHex(recipientPrivateKeyHex))
		require.Error(t, err)
		require.Nil(t, address)
	})

	t.Run("return error when nil is passed as recipient private key deriver", func(t *testing.T) {
		var keyDeriver *sdk.KeyDeriver
		address, err := brc29.AddressForSelf(brc29.PubHex(senderPublicKeyHex), keyID, keyDeriver)
		require.Error(t, err)
		require.Nil(t, address)
	})

	t.Run("return error when nil is passed as recipient private key", func(t *testing.T) {
		var priv *ec.PrivateKey
		address, err := brc29.AddressForSelf(brc29.PubHex(senderPublicKeyHex), keyID, priv)
		require.Error(t, err)
		require.Nil(t, address)
	})
}

func TestBRC29AddressCreation(t *testing.T) {
	t.Run("return valid address created with brc28 with hex string as sender private key source", func(t *testing.T) {
		// when:
		address, err := brc29.AddressForCounterparty(brc29.PrivHex(senderPrivateKeyHex), keyID, brc29.PubHex(recipientPublicKeyHex))

		// then:
		require.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedAddress, address.AddressString)
	})

	t.Run("return valid address created with brc28 with wif as sender private key source", func(t *testing.T) {
		// when:
		address, err := brc29.AddressForCounterparty(brc29.WIF(senderWIFString), keyID, brc29.PubHex(recipientPublicKeyHex))

		// then:
		require.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedAddress, address.AddressString)
	})

	t.Run("return valid address created with brc28 with ec.PrivateKey as sender private key source", func(t *testing.T) {
		// given:
		priv, err := ec.PrivateKeyFromHex(senderPrivateKeyHex)
		require.NoError(t, err)

		// when:
		address, err := brc29.AddressForCounterparty(priv, keyID, brc29.PubHex(recipientPublicKeyHex))

		// then:
		require.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedAddress, address.AddressString)
	})

	t.Run("return valid address created with brc28 with key deriver as sender private key source", func(t *testing.T) {
		// given:
		priv, err := ec.PrivateKeyFromHex(senderPrivateKeyHex)
		require.NoError(t, err)

		keyDeriver := sdk.NewKeyDeriver(priv)

		// when:
		address, err := brc29.AddressForCounterparty(keyDeriver, keyID, brc29.PubHex(recipientPublicKeyHex))

		// then:
		require.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedAddress, address.AddressString)
	})

	t.Run("return valid address created with brc28 with ec.PublicKey as receiver public key source", func(t *testing.T) {
		// given:
		pub, err := ec.PublicKeyFromString(recipientPublicKeyHex)
		require.NoError(t, err)

		// when:
		address, err := brc29.AddressForCounterparty(brc29.PrivHex(senderPrivateKeyHex), keyID, pub)

		// then:
		require.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedAddress, address.AddressString)
	})

	t.Run("return testnet address created with brc29", func(t *testing.T) {
		// given:
		pub, err := ec.PublicKeyFromString(recipientPublicKeyHex)
		require.NoError(t, err)

		// when:
		address, err := brc29.AddressForCounterparty(brc29.PrivHex(senderPrivateKeyHex), keyID, pub, brc29.WithTestNet())

		// then:
		require.NoError(t, err)
		require.NotNil(t, address)
		require.Equal(t, expectedTestnetAddress, address.AddressString)
	})
}

func TestBRC29AddressErrors(t *testing.T) {
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
			// when:
			address, err := brc29.AddressForCounterparty(brc29.PrivHex(test.sender), test.keyID, brc29.PubHex(test.recipient))

			// then:
			require.Nil(t, address)
			require.Error(t, err)
		})
	}

	t.Run("return error when nil is passed as sender private key deriver", func(t *testing.T) {
		// given:
		var keyDeriver *sdk.KeyDeriver

		// when:
		address, err := brc29.AddressForCounterparty(keyDeriver, keyID, brc29.PubHex(recipientPublicKeyHex))

		// then:
		require.Error(t, err)
		require.Nil(t, address)
	})

	t.Run("return error when nil is passed as sender private key", func(t *testing.T) {
		// given:
		var priv *ec.PrivateKey

		// when:
		address, err := brc29.AddressForCounterparty(priv, keyID, brc29.PubHex(recipientPublicKeyHex))

		// then:
		require.Error(t, err)
		require.Nil(t, address)
	})

	t.Run("return error when nil is passed as recipient public key deriver", func(t *testing.T) {
		// given:
		var keyDeriver *sdk.KeyDeriver

		// when:
		address, err := brc29.AddressForCounterparty(brc29.PrivHex(senderPrivateKeyHex), keyID, keyDeriver)

		// then:
		require.Error(t, err)
		require.Nil(t, address)
	})

	t.Run("return error when nil is passed as recipient public key", func(t *testing.T) {
		// given:
		var pub *ec.PublicKey

		// when:
		address, err := brc29.AddressForCounterparty(brc29.PrivHex(senderPrivateKeyHex), keyID, pub)

		// then:
		require.Error(t, err)
		require.Nil(t, address)
	})
}
