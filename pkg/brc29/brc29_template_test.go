package brc29_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script/interpreter"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
)

const mustGetAddressMsg = "Must get address from BRC29 locking script"

func TestBRC29TemplateLock(t *testing.T) {
	t.Run("should lock with P2PKH and BRC29 calculated address", func(t *testing.T) {
		// when:
		lockingScript, err := brc29.LockForCounterparty(brc29.PrivHex(senderPrivateKeyHex), keyID, brc29.PubHex(recipientPublicKeyHex))
		// then:
		require.NoError(t, err)
		require.NotNil(t, lockingScript)

		// and:
		address, err := lockingScript.Address()
		require.NoError(t, err, mustGetAddressMsg)
		require.NotNil(t, address, mustGetAddressMsg)

		assert.Equal(t, expectedAddress, address.AddressString)
	})

	t.Run("return error when nil is passed as sender private key deriver", func(t *testing.T) {
		// given:
		var keyDeriver *sdk.KeyDeriver

		// when:
		lockingScript, err := brc29.LockForCounterparty(keyDeriver, keyID, brc29.PubHex(recipientPublicKeyHex))

		// then:
		require.Error(t, err)
		require.Nil(t, lockingScript)
	})

	t.Run("return error when nil is passed as sender private key", func(t *testing.T) {
		// given:
		var priv *ec.PrivateKey

		// when:
		lockingScript, err := brc29.LockForCounterparty(priv, keyID, brc29.PubHex(recipientPublicKeyHex))

		// then:
		require.Error(t, err)
		require.Nil(t, lockingScript)
	})

	t.Run("return error when nil is passed as recipient public key deriver", func(t *testing.T) {
		// given:
		var keyDeriver *sdk.KeyDeriver

		// when:
		lockingScript, err := brc29.LockForCounterparty(brc29.PrivHex(senderPrivateKeyHex), keyID, keyDeriver)

		// then:
		require.Error(t, err)
		require.Nil(t, lockingScript)
	})

	t.Run("return error when nil is passed as recipient public key", func(t *testing.T) {
		// given:
		var pub *ec.PublicKey

		// when:
		lockingScript, err := brc29.LockForCounterparty(brc29.PrivHex(senderPrivateKeyHex), keyID, pub)

		// then:
		require.Error(t, err)
		require.Nil(t, lockingScript)
	})

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
			lockingScript, err := brc29.LockForCounterparty(brc29.PrivHex(test.sender), test.keyID, brc29.PubHex(test.recipient))

			// then:
			require.Nil(t, lockingScript)
			require.Error(t, err)
		})
	}
}

func TestBRC29TemplateLockForSelf(t *testing.T) {
	t.Run("should lock with P2PKH and BRC29 calculated address (self)", func(t *testing.T) {
		// when:
		lockingScript, err := brc29.LockForSelf(brc29.PubHex(senderPublicKeyHex), keyID, brc29.PrivHex(recipientPrivateKeyHex))
		// then:
		require.NoError(t, err)
		require.NotNil(t, lockingScript)

		// and:
		address, err := lockingScript.Address()
		require.NoError(t, err, mustGetAddressMsg)
		require.NotNil(t, address, mustGetAddressMsg)

		assert.Equal(t, expectedAddress, address.AddressString)
	})

	t.Run("return error when nil is passed as sender public key deriver", func(t *testing.T) {
		// given:
		var keyDeriver *sdk.KeyDeriver

		// when:
		lockingScript, err := brc29.LockForSelf(keyDeriver, keyID, brc29.PrivHex(recipientPrivateKeyHex))

		// then:
		require.Error(t, err)
		require.Nil(t, lockingScript)
	})

	t.Run("return error when nil is passed as sender public key", func(t *testing.T) {
		// given:
		var pub *ec.PublicKey

		// when:
		lockingScript, err := brc29.LockForSelf(pub, keyID, brc29.PrivHex(recipientPrivateKeyHex))

		// then:
		require.Error(t, err)
		require.Nil(t, lockingScript)
	})

	t.Run("return error when nil is passed as recipient private key deriver", func(t *testing.T) {
		// given:
		var keyDeriver *sdk.KeyDeriver

		// when:
		lockingScript, err := brc29.LockForSelf(brc29.PubHex(senderPublicKeyHex), keyID, keyDeriver)

		// then:
		require.Error(t, err)
		require.Nil(t, lockingScript)
	})

	t.Run("return error when nil is passed as recipient private key", func(t *testing.T) {
		// given:
		var priv *ec.PrivateKey

		// when:
		lockingScript, err := brc29.LockForSelf(brc29.PubHex(senderPublicKeyHex), keyID, priv)

		// then:
		require.Error(t, err)
		require.Nil(t, lockingScript)
	})

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
			// when:
			lockingScript, err := brc29.LockForSelf(brc29.PubHex(test.sender), test.keyID, brc29.PrivHex(test.recipient))

			// then:
			require.Nil(t, lockingScript)
			require.Error(t, err)
		})
	}
}

func TestBRC29TemplateUnlock(t *testing.T) {
	t.Run("unlock the output locked with BRC29 locker", func(t *testing.T) {
		// given:
		prevTxID, err := chainhash.NewHashFromHex("64faeaa2e3cbadaf82d8fa8c7ded508cb043c5d101671f43c084be2ac6163148")
		require.NoError(t, err)

		// and:
		tx := transaction.NewTransaction()
		err = tx.AddOpReturnOutput([]byte("anything"))
		require.NoError(t, err)

		// and:
		lockingScript, err := brc29.LockForCounterparty(brc29.PrivHex(senderPrivateKeyHex), keyID, brc29.PubHex(recipientPublicKeyHex))
		require.NoError(t, err)

		// when:
		unlocker, err := brc29.Unlock(brc29.PubHex(senderPublicKeyHex), keyID, brc29.PrivHex(recipientPrivateKeyHex))
		require.NoError(t, err)

		// and:
		utxo := &transaction.UTXO{
			TxID:                    prevTxID,
			Vout:                    0,
			Satoshis:                1000,
			LockingScript:           lockingScript,
			UnlockingScriptTemplate: unlocker,
		}

		err = tx.AddInputsFromUTXOs(utxo)
		require.NoError(t, err)

		// and:
		err = tx.Sign()
		require.NoError(t, err)

		// then:
		err = interpreter.NewEngine().Execute(interpreter.WithTx(tx, 0, tx.Inputs[0].SourceTxOutput()),
			interpreter.WithAfterGenesis(),
			interpreter.WithForkID())

		require.NoError(t, err)
	})

	t.Run("estimate unlocking script length", func(t *testing.T) {
		unlocker, err := brc29.Unlock(brc29.PubHex(senderPublicKeyHex), keyID, brc29.PrivHex(recipientPrivateKeyHex))
		require.NoError(t, err)

		length := unlocker.EstimateLength(transaction.NewTransaction(), 0)

		assert.EqualValues(t, 106, length)
	})

	errorTestCases := map[string]struct {
		sender    string
		keyID     brc29.KeyID
		recipient string
	}{
		"return error when sender key is empty": {
			sender:    "",
			keyID:     keyID,
			recipient: recipientPrivateKeyHex,
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
			// when:
			unlocker, err := brc29.Unlock(brc29.PubHex(test.sender), test.keyID, brc29.PrivHex(test.recipient))

			// then:
			require.Nil(t, unlocker)
			require.Error(t, err)
		})
	}
}
