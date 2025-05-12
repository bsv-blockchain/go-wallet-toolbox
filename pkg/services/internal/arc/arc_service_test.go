package arc_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/results"
	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostBEEFWithARCService(t *testing.T) {

	t.Run("broadcast without passing txid", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)

		// setup arc server
		given.ARC().IsUpAndRunning()

		// and:
		service := given.Services().NewArcService()

		// and:
		tx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
		beef, err := sdk.NewBeefFromTransaction(tx)
		require.NoError(t, err)

		txID := tx.TxID().String()

		// when:
		res, err := service.PostBeef(context.Background(), beef, nil)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, res)

		require.ElementsMatch(t,
			res.TxIDResults,
			[]results.PostTxID{
				{
					Result: results.ResultStatusSuccess,
					TxID:   tx.TxID().String(),
					Data:   given.ARC().TxInfoJSON(txID),
				},
			})
	})

	t.Run("broadcast single transaction", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)

		// setup arc server
		given.ARC().IsUpAndRunning()

		// and:
		service := given.Services().NewArcService()

		// and:
		tx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
		beef, err := sdk.NewBeefFromTransaction(tx)
		require.NoError(t, err)

		txID := tx.TxID().String()
		var txids = []string{txID}

		// when:
		res, err := service.PostBeef(context.Background(), beef, txids)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, res)

		require.ElementsMatch(t,
			res.TxIDResults,
			[]results.PostTxID{
				{
					Result: results.ResultStatusSuccess,
					TxID:   tx.TxID().String(),
					Data:   given.ARC().TxInfoJSON(txID),
				},
			})
	})

	t.Run("broadcast multiple txids", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)

		// setup arc server
		given.ARC().IsUpAndRunning()

		// and:
		service := given.Services().NewArcService()

		// and:
		parentTx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
		parentTxID := parentTx.TxID().String()

		// and:
		childTx := txtestabilities.GivenTX().WithInputFromUTXO(parentTx, 0).WithP2PKHOutput(98).TX()
		childTxID := childTx.TxID().String()
		beef, err := sdk.NewBeefFromTransaction(childTx)
		require.NoError(t, err)

		var txids = []string{parentTxID, childTxID}

		// when:
		res, err := service.PostBeef(context.Background(), beef, txids)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, res)

		require.ElementsMatch(t,
			res.TxIDResults,
			[]results.PostTxID{
				{
					Result: results.ResultStatusSuccess,
					TxID:   parentTx.TxID().String(),
					Data:   given.ARC().TxInfoJSON(parentTxID),
				},
				{
					Result: results.ResultStatusSuccess,
					TxID:   childTxID,
					Data:   given.ARC().TxInfoJSON(childTxID),
				},
			})
	})

	t.Run("return success if broadcast finished with OK without body, but we can query the tx", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)

		// setup arc server
		given.ARC().IsUpAndRunning()
		given.ARC().OnBroadcast().WillReturnNoBody()

		// and:
		service := given.Services().NewArcService()

		// and:
		tx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
		beef, err := sdk.NewBeefFromTransaction(tx)
		require.NoError(t, err)

		txID := tx.TxID().String()
		var txids = []string{txID}

		// when:
		res, err := service.PostBeef(context.Background(), beef, txids)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, res)

		require.ElementsMatch(t,
			res.TxIDResults,
			[]results.PostTxID{
				{
					Result: results.ResultStatusSuccess,
					TxID:   tx.TxID().String(),
					Data:   given.ARC().TxInfoJSON(txID),
				},
			})
	})

	invalidBEEFTestCases := map[string]struct {
		BEEF func(t testing.TB) *sdk.Beef
	}{
		"return error on nil beef": {
			BEEF: func(t testing.TB) *sdk.Beef {
				return nil
			},
		},
		"return error on empty beef": {
			BEEF: func(t testing.TB) *sdk.Beef {
				return sdk.NewBeefV2()
			},
		},
		"return error on beef v2 with txID only": {
			BEEF: func(t testing.TB) *sdk.Beef {
				tx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
				beef, err := sdk.NewBeefFromTransaction(tx)
				require.NoError(t, err)

				beefTxIDOnly, err := beef.TxidOnly()
				require.NoError(t, err)
				return beefTxIDOnly
			},
		},
		"return error on beef v2 with multiple subject transactions (TEMPORARY)": {
			BEEF: func(t testing.TB) *sdk.Beef {
				tx1 := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
				beef, err := sdk.NewBeefFromTransaction(tx1)
				require.NoError(t, err)

				tx2 := txtestabilities.GivenTX().WithInput(200).WithP2PKHOutput(100).TX()

				_, err = beef.MergeTransaction(tx2)
				require.NoError(t, err)

				return beef
			},
		},
	}
	for name, test := range invalidBEEFTestCases {
		t.Run(name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)

			// setup arc server
			given.ARC().IsUpAndRunning()

			// and:
			service := given.Services().NewArcService()

			// when:
			res, err := service.PostBeef(context.Background(), test.BEEF(t), nil)

			// then:
			assert.Error(t, err)
			assert.Nil(t, res)
		})
	}

	arcFailingTestCases := map[string]struct {
		setupARC func(testabilities.ARCFixture)
	}{
		"return error when arc is unreachable": {
			setupARC: func(testabilities.ARCFixture) {},
		},
		"return error when arc returns unauthorized": {
			setupARC: func(arc testabilities.ARCFixture) {
				arc.WillAlwaysReturnStatus(http.StatusUnauthorized)
			},
		},
		"return error when arc returns forbidden": {
			setupARC: func(arc testabilities.ARCFixture) {
				arc.WillAlwaysReturnStatus(http.StatusForbidden)
			},
		},
		"return error when arc returns not found": {
			setupARC: func(arc testabilities.ARCFixture) {
				arc.WillAlwaysReturnStatus(http.StatusNotFound)
			},
		},
		"return error when arc returns internal server error": {
			setupARC: func(arc testabilities.ARCFixture) {
				arc.WillAlwaysReturnStatus(http.StatusInternalServerError)
			},
		},
	}
	for name, test := range arcFailingTestCases {
		t.Run(name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)

			// setup arc server
			test.setupARC(given.ARC())

			// and:
			service := given.Services().NewArcService()

			// and:
			tx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
			beef, err := sdk.NewBeefFromTransaction(tx)
			require.NoError(t, err)

			// when:
			res, err := service.PostBeef(context.Background(), beef, nil)

			// then:
			assert.Error(t, err)
			assert.Nil(t, res)
		})
	}

	errorOnQueryTxTestCases := map[string]struct {
		setupARCQueryTx func(arc testabilities.ARCQueryFixture)
	}{
		"return success result when broadcast succeed but getting info about tx failed because arc is unreachable": {
			setupARCQueryTx: func(arc testabilities.ARCQueryFixture) {
				arc.WillBeUnreachable()
			},
		},
		"return success result when broadcast succeed but getting info about tx failed with unauthorized": {
			setupARCQueryTx: func(arc testabilities.ARCQueryFixture) {
				arc.WillReturnHttpStatus(http.StatusUnauthorized)
			},
		},
		"return success result when broadcast succeed but getting info about tx failed with forbidden": {
			setupARCQueryTx: func(arc testabilities.ARCQueryFixture) {
				arc.WillReturnHttpStatus(http.StatusForbidden)
			},
		},
		"return success result when broadcast succeed but getting info about tx failed with conflict": {
			setupARCQueryTx: func(arc testabilities.ARCQueryFixture) {
				arc.WillReturnHttpStatus(http.StatusConflict)
			},
		},
		"return success result when broadcast succeed but getting info about tx failed with internal server error": {
			setupARCQueryTx: func(arc testabilities.ARCQueryFixture) {
				arc.WillReturnHttpStatus(http.StatusInternalServerError)
			},
		},
		"return success result when broadcast succeed but getting info about tx failed with not found": {
			setupARCQueryTx: func(arc testabilities.ARCQueryFixture) {
				arc.WillReturnHttpStatus(http.StatusNotFound)
			},
		},
		"return success result when broadcast succeed but getting info would result with no body": {
			setupARCQueryTx: func(arc testabilities.ARCQueryFixture) {
				arc.WillReturnNoBody()
			},
		},
		"return success result when broadcast succeed but getting info would result with different transaction": {
			setupARCQueryTx: func(arc testabilities.ARCQueryFixture) {
				arc.WillReturnDifferentTxID()
			},
		},
	}
	for name, test := range errorOnQueryTxTestCases {
		t.Run(name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)

			// setup arc server
			given.ARC().IsUpAndRunning()

			// and:
			service := given.Services().NewArcService()

			// and:
			grandParentTx := txtestabilities.GivenTX().WithInput(300).WithP2PKHOutput(299).TX()
			grandParentTxID := grandParentTx.TxID().String()

			parentTx := txtestabilities.GivenTX().WithInputFromUTXO(grandParentTx, 0).WithP2PKHOutput(199).TX()
			parentTxID := parentTx.TxID().String()

			// and:
			childTx := txtestabilities.GivenTX().WithInputFromUTXO(parentTx, 0).WithP2PKHOutput(99).TX()
			childTxID := childTx.TxID().String()
			beef, err := sdk.NewBeefFromTransaction(childTx)
			require.NoError(t, err)

			var txids = []string{grandParentTxID, parentTxID, childTxID}

			// and:
			test.setupARCQueryTx(given.ARC().WhenQueryingTx(parentTxID))

			// when:
			res, err := service.PostBeef(context.Background(), beef, txids)

			// then:
			assert.NoError(t, err)
			require.NotNil(t, res)

			for _, resultForTxID := range res.TxIDResults {
				if resultForTxID.TxID == parentTxID {
					assert.Equal(t, results.ResultStatusError, resultForTxID.Result, "expect (parentTx) tx %s to have error result", resultForTxID.TxID)
					assert.Nil(t, resultForTxID.Data, "expect result for (parentTx) tx %s to have no data", resultForTxID.TxID)
					assert.Error(t, resultForTxID.Error, "expect result for (parentTx) tx %s to have an error", resultForTxID.TxID)
				} else {
					assert.Equal(t, results.ResultStatusSuccess, resultForTxID.Result, "expect tx %s to have success result", resultForTxID.TxID)
				}
			}
		})
	}
}
