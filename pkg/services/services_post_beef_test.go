package services_test

import (
	"context"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostBEEF(t *testing.T) {
	t.Run("successfully post PostedBEEF with single tx IDs", func(t *testing.T) {
		// given:
		given := testabilities.GivenServices(t)
		given.ARC().IsUpAndRunning()

		// and:
		tx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
		beef, err := sdk.NewBeefFromTransaction(tx)
		require.NoError(t, err)

		txID := tx.TxID().String()
		var txids = []string{txID}

		// and:
		services := given.Services().WithDefaultConfig()

		// when:
		response, err := services.PostBEEF(context.Background(), beef, txids)

		// then:
		assert.NoError(t, err)
		assert.NotEmpty(t, response)

		slices.ForEach(response, func(item *wdk.PostBeefSingleResult) {
			assert.NotEmpty(t, item.Name)
			assert.NoError(t, item.Error)
			if assert.NotNil(t, item.PostedBEEF) {
				result := item.PostedBEEF
				assert.Lenf(t, result.TxIDResults, len(txids), "service %s returned unexpected number of results", item.Name)
			}
		})
	})

	t.Run("successfully post PostedBEEF with multiple tx IDs", func(t *testing.T) {
		// given:
		given := testabilities.GivenServices(t)
		given.ARC().IsUpAndRunning()

		// and:
		parentTx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
		parentTxID := parentTx.TxID().String()

		// and:
		childTx := txtestabilities.GivenTX().WithInputFromUTXO(parentTx, 0).WithP2PKHOutput(98).TX()
		childTxID := childTx.TxID().String()
		beef, err := sdk.NewBeefFromTransaction(childTx)
		require.NoError(t, err)

		var txids = []string{parentTxID, childTxID}

		// and:
		services := given.Services().WithDefaultConfig()

		// when:
		response, err := services.PostBEEF(context.Background(), beef, txids)

		// then:
		assert.NoError(t, err)
		assert.NotEmpty(t, response)

		slices.ForEach(response, func(item *wdk.PostBeefSingleResult) {
			assert.NotEmpty(t, item.Name)
			assert.NoError(t, item.Error)
			if assert.NotNil(t, item.PostedBEEF) {
				result := item.PostedBEEF
				assert.Lenf(t, result.TxIDResults, len(txids), "service %s returned unexpected number of results", item.Name)
			}
		})
	})

}
