package services_test

import (
	"testing"

	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
)

func TestGetBeef(t *testing.T) {
	t.Run("return error when all services are unreachable", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		// and:
		givenGetBeefFixture := given.WhatsOnChain().MinedTransaction()
		txID := givenGetBeefFixture.TxID()

		// and:
		services := given.Services().New()

		// when:
		beef, err := services.GetBEEF(t.Context(), txID, nil)

		// then:
		require.Error(t, err)
		assert.Nil(t, beef)
	})

	t.Run("return result without Merkle Path when transaction is not mined yet", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		// and:
		givenMinedTx := given.WhatsOnChain().MinedTransaction()
		givenMinedTx.WillReturnRawTx()
		txID := givenMinedTx.TxID()

		// and:
		services := given.Services().New()

		// when:
		_, err := services.GetBEEF(t.Context(), txID, nil)

		// then:
		assert.Error(t, err)
	})

	t.Run("for mined transaction", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		givenMinedTx := given.WhatsOnChain().MinedTransaction()
		givenMinedTx.WillReturnRawTx()
		givenMinedTx.WillReturnMerklePath()
		txID := givenMinedTx.TxID()

		// and:
		services := given.Services().New()

		// when:
		beef, err := services.GetBEEF(t.Context(), txID, nil)

		// then:
		require.NoError(t, err)
		tx := beef.FindTransaction(txID)
		assert.NotNil(t, tx)
		assert.NotNil(t, tx.MerklePath)
	})

	t.Run("parent is mined and child is not but has rawTx", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		givenMinedTx := given.WhatsOnChain().MinedTransaction()
		givenMinedTx.WillReturnRawTx()
		givenMinedTx.WillReturnMerklePath()

		childTxSpec := testvectors.GivenTX().
			WithInputFromUTXO(givenMinedTx.Tx(), 0).
			WithP2PKHOutput(1)

		given.WhatsOnChain().WillRespondWithRawTx(200, childTxSpec.ID().String(), childTxSpec.RawTX().Hex(), nil)
		given.WhatsOnChain().WillRespondWithMerklePath(404, childTxSpec.ID().String(), "")

		// and:
		services := given.Services().New()

		// when:
		beef, err := services.GetBEEF(t.Context(), childTxSpec.ID().String(), nil)

		// then:
		require.NoError(t, err)
		childTx := beef.FindTransaction(childTxSpec.ID().String())
		assert.NotNil(t, childTx)
		assert.Equal(t, childTxSpec.RawTX().Hex(), childTx.Hex())

		parentTx := beef.FindTransaction(givenMinedTx.TxID())
		assert.NotNil(t, parentTx)
		assert.NotNil(t, parentTx.MerklePath)
	})

	t.Run("child tx is not mined but the parent is passed as knownTxID", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		givenMinedTx := given.WhatsOnChain().MinedTransaction()

		childTxSpec := testvectors.GivenTX().
			WithInputFromUTXO(givenMinedTx.Tx(), 0).
			WithP2PKHOutput(1)

		given.WhatsOnChain().WillRespondWithRawTx(200, childTxSpec.ID().String(), childTxSpec.RawTX().Hex(), nil)
		given.WhatsOnChain().WillRespondWithMerklePath(404, childTxSpec.ID().String(), "")

		// and:
		services := given.Services().New()

		// when:
		beef, err := services.GetBEEF(t.Context(), childTxSpec.ID().String(), []string{givenMinedTx.TxID()})

		// then:
		require.NoError(t, err)
		childTx := beef.FindTransaction(childTxSpec.ID().String())
		assert.NotNil(t, childTx)
		assert.Equal(t, childTxSpec.RawTX().Hex(), childTx.Hex())

		parentTx := beef.FindTransaction(givenMinedTx.TxID())
		assert.Nil(t, parentTx)
	})
}
