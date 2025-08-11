package storage_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBeef(t *testing.T) {
	t.Run("empty storage, fetched from services, the tx has merkle path", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		givenMinedTx := given.Provider().WhatsOnChain().MinedTransaction()
		givenMinedTx.WillReturnRawTx()
		givenMinedTx.WillReturnMerklePath()
		txID := givenMinedTx.TxID()

		// and:
		activeStorage := given.Provider().GORM()

		// when:
		beef, err := activeStorage.GetBeefForTransaction(t.Context(), txID, wdk.StorageGetBeefOptions{})

		// then:
		require.NoError(t, err)
		require.NotNil(t, beef)

		assert.NotNil(t, beef.FindTransaction(txID))
	})

	t.Run("storage has parent transaction (mined), child tx needs to be fetched from services", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		activeStorage := given.Provider().GORM()

		parentTx := given.Provider().WhatsOnChain().MinedTransaction().Tx()

		atomicBeef, err := parentTx.AtomicBEEF(false)
		require.NoError(t, err)

		given.Provider().BHS().OnMerkleRootVerifyResponse(
			1687775,
			"6861d579c2fb885c2fef10ce39c2750d9b50c4185727b19989de657fa105d1b7",
			testabilities.BHSMerkleRootConfirmed,
		)

		args := fixtures.DefaultInternalizeActionArgs(t, wdk.BasketInsertionProtocol)
		args.Tx = atomicBeef

		_, err = activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)
		require.NoError(t, err)

		// and:
		childTxSpec := testvectors.GivenTX().WithInputFromUTXO(parentTx, 0).WithP2PKHOutput(1)
		childTxID := childTxSpec.ID().String()
		given.Provider().WhatsOnChain().WillRespondWithRawTx(200, childTxID, childTxSpec.RawTX().Hex(), nil)
		given.Provider().WhatsOnChain().WillRespondWithMerklePath(404, childTxID, "")

		// when:
		beef, err := activeStorage.GetBeefForTransaction(t.Context(), childTxID, wdk.StorageGetBeefOptions{})

		// then:
		require.NoError(t, err)
		require.NotNil(t, beef)

		assert.NotNil(t, beef.FindTransaction(childTxID))
		assert.NotNil(t, beef.FindTransaction(parentTx.TxID().String()))
	})

	t.Run("ignoreStorage option, fetched from services, the tx has merkle path", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		givenMinedTx := given.Provider().WhatsOnChain().MinedTransaction()
		givenMinedTx.WillReturnRawTx()
		givenMinedTx.WillReturnMerklePath()
		txID := givenMinedTx.TxID()

		activeStorage := given.Provider().GORM()

		// when:
		beef, err := activeStorage.GetBeefForTransaction(t.Context(), txID, wdk.StorageGetBeefOptions{IgnoreStorage: true})

		// then:
		require.NoError(t, err)
		require.NotNil(t, beef)

		assert.NotNil(t, beef.FindTransaction(txID))
	})

	t.Run("empty storage, fetched from services, should fail, the tx doesn't have merkle path", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		givenMinedTx := given.Provider().WhatsOnChain().MinedTransaction()
		givenMinedTx.WillReturnRawTx()
		txID := givenMinedTx.TxID()

		given.Provider().WhatsOnChain().WillRespondWithMerklePath(404, txID, "")

		activeStorage := given.Provider().GORM()

		// when:
		_, err := activeStorage.GetBeefForTransaction(t.Context(), txID, wdk.StorageGetBeefOptions{})

		// then:
		require.Error(t, err)
	})
}
