package arc_test

import (
	"net/http"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	arctestabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arc/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostBEEFWithARCService(t *testing.T) {

	t.Run("broadcast without passing txid", func(t *testing.T) {
		// given:
		given := arctestabilities.Given(t)

		// setup arc server
		given.ARC().IsUpAndRunning()

		// and:
		service := given.NewArcService()

		// and:
		tx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
		beef, err := sdk.NewBeefFromTransaction(tx)
		require.NoError(t, err)

		txID := tx.TxID().String()

		// when:
		res, err := service.PostBEEF(t.Context(), beef, nil)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, res)

		require.ElementsMatch(t,
			res.TxIDResults,
			[]wdk.PostedTxID{
				{
					Result: wdk.PostedTxIDResultSuccess,
					TxID:   tx.TxID().String(),
					Data:   given.ARC().TxInfoJSON(txID),
				},
			})
	})

	t.Run("broadcast single transaction", func(t *testing.T) {
		// given:
		given := arctestabilities.Given(t)

		// setup arc server
		given.ARC().IsUpAndRunning()

		// and:
		service := given.NewArcService()

		// and:
		tx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
		beef, err := sdk.NewBeefFromTransaction(tx)
		require.NoError(t, err)

		txID := tx.TxID().String()
		var txids = []string{txID}

		// when:
		res, err := service.PostBEEF(t.Context(), beef, txids)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, res)

		require.ElementsMatch(t,
			res.TxIDResults,
			[]wdk.PostedTxID{
				{
					Result: wdk.PostedTxIDResultSuccess,
					TxID:   tx.TxID().String(),
					Data:   given.ARC().TxInfoJSON(txID),
				},
			})
	})

	t.Run("broadcast multiple txids", func(t *testing.T) {
		// given:
		given := arctestabilities.Given(t)

		// setup arc server
		given.ARC().IsUpAndRunning()

		// and:
		service := given.NewArcService()

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
		res, err := service.PostBEEF(t.Context(), beef, txids)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, res)

		require.ElementsMatch(t,
			res.TxIDResults,
			[]wdk.PostedTxID{
				{
					Result: wdk.PostedTxIDResultSuccess,
					TxID:   parentTx.TxID().String(),
					Data:   given.ARC().TxInfoJSON(parentTxID),
				},
				{
					Result: wdk.PostedTxIDResultSuccess,
					TxID:   childTxID,
					Data:   given.ARC().TxInfoJSON(childTxID),
				},
			})
	})

	t.Run("return success if broadcast finished with OK without body, but we can query the tx", func(t *testing.T) {
		// given:
		given := arctestabilities.Given(t)

		// setup arc server
		given.ARC().IsUpAndRunning()
		given.ARC().OnBroadcast().WillReturnNoBody()

		// and:
		service := given.NewArcService()

		// and:
		tx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
		beef, err := sdk.NewBeefFromTransaction(tx)
		require.NoError(t, err)

		txID := tx.TxID().String()
		var txids = []string{txID}

		// when:
		res, err := service.PostBEEF(t.Context(), beef, txids)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, res)

		require.ElementsMatch(t,
			res.TxIDResults,
			[]wdk.PostedTxID{
				{
					Result: wdk.PostedTxIDResultSuccess,
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
			given := arctestabilities.Given(t)

			// setup arc server
			given.ARC().IsUpAndRunning()

			// and:
			service := given.NewArcService()

			// when:
			res, err := service.PostBEEF(t.Context(), test.BEEF(t), nil)

			// then:
			assert.Error(t, err)
			assert.Nil(t, res)
		})
	}

	arcFailingTestCases := map[string]struct {
		setupARC func(testservices.ARCFixture)
	}{
		"return error when arc is unreachable": {
			setupARC: func(testservices.ARCFixture) {},
		},
		"return error when arc returns unauthorized": {
			setupARC: func(arc testservices.ARCFixture) {
				arc.WillAlwaysReturnStatus(http.StatusUnauthorized)
			},
		},
		"return error when arc returns forbidden": {
			setupARC: func(arc testservices.ARCFixture) {
				arc.WillAlwaysReturnStatus(http.StatusForbidden)
			},
		},
		"return error when arc returns not found": {
			setupARC: func(arc testservices.ARCFixture) {
				arc.WillAlwaysReturnStatus(http.StatusNotFound)
			},
		},
		"return error when arc returns internal server error": {
			setupARC: func(arc testservices.ARCFixture) {
				arc.WillAlwaysReturnStatus(http.StatusInternalServerError)
			},
		},
	}
	for name, test := range arcFailingTestCases {
		t.Run(name, func(t *testing.T) {
			// given:
			given := arctestabilities.Given(t)

			// setup arc server
			test.setupARC(given.ARC())

			// and:
			service := given.NewArcService()

			// and:
			tx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
			beef, err := sdk.NewBeefFromTransaction(tx)
			require.NoError(t, err)

			// when:
			res, err := service.PostBEEF(t.Context(), beef, nil)

			// then:
			assert.Error(t, err)
			assert.Nil(t, res)
		})
	}

	errorOnQueryTxTestCases := map[string]struct {
		setupARCQueryTx func(arc testservices.ARCQueryFixture)
	}{
		"return success result when broadcast succeed but getting info about tx failed because arc is unreachable": {
			setupARCQueryTx: func(arc testservices.ARCQueryFixture) {
				arc.WillBeUnreachable()
			},
		},
		"return success result when broadcast succeed but getting info about tx failed with unauthorized": {
			setupARCQueryTx: func(arc testservices.ARCQueryFixture) {
				arc.WillReturnHttpStatus(http.StatusUnauthorized)
			},
		},
		"return success result when broadcast succeed but getting info about tx failed with forbidden": {
			setupARCQueryTx: func(arc testservices.ARCQueryFixture) {
				arc.WillReturnHttpStatus(http.StatusForbidden)
			},
		},
		"return success result when broadcast succeed but getting info about tx failed with conflict": {
			setupARCQueryTx: func(arc testservices.ARCQueryFixture) {
				arc.WillReturnHttpStatus(http.StatusConflict)
			},
		},
		"return success result when broadcast succeed but getting info about tx failed with internal server error": {
			setupARCQueryTx: func(arc testservices.ARCQueryFixture) {
				arc.WillReturnHttpStatus(http.StatusInternalServerError)
			},
		},
		"return success result when broadcast succeed but getting info about tx failed with not found": {
			setupARCQueryTx: func(arc testservices.ARCQueryFixture) {
				arc.WillReturnHttpStatus(http.StatusNotFound)
			},
		},
		"return success result when broadcast succeed but getting info would result with no body": {
			setupARCQueryTx: func(arc testservices.ARCQueryFixture) {
				arc.WillReturnNoBody()
			},
		},
		"return success result when broadcast succeed but getting info would result with different transaction": {
			setupARCQueryTx: func(arc testservices.ARCQueryFixture) {
				arc.WillReturnDifferentTxID()
			},
		},
	}
	for name, test := range errorOnQueryTxTestCases {
		t.Run(name, func(t *testing.T) {
			// given:
			given := arctestabilities.Given(t)

			// setup arc server
			given.ARC().IsUpAndRunning()

			// and:
			service := given.NewArcService()

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
			res, err := service.PostBEEF(t.Context(), beef, txids)

			// then:
			assert.NoError(t, err)
			require.NotNil(t, res)

			for _, resultForTxID := range res.TxIDResults {
				if resultForTxID.TxID == parentTxID {
					assert.Equal(t, wdk.PostedTxIDResultError, resultForTxID.Result, "expect (parentTx) tx %s to have error result", resultForTxID.TxID)
					assert.Empty(t, resultForTxID.Data, "expect result for (parentTx) tx %s to have no data", resultForTxID.TxID)
					assert.Error(t, resultForTxID.Error, "expect result for (parentTx) tx %s to have an error", resultForTxID.TxID)
				} else {
					assert.Equal(t, wdk.PostedTxIDResultSuccess, resultForTxID.Result, "expect tx %s to have success result", resultForTxID.TxID)
				}
			}
		})
	}
}

func TestGetMerklePathWithARCService(t *testing.T) {
	tx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
	txID := tx.TxID().String()

	someSecondHash, errHash := chainhash.NewHashFromHex("27a53423aa3e5d5c46bf30be53a9998dd247daf758847f244f82d430be71de6e")
	require.NoError(t, errHash)

	arcErrorTestCases := map[string]struct {
		setupARC func(arc testservices.ARCFixture)
	}{
		"return error when arc is unreachable": {
			setupARC: func(arc testservices.ARCFixture) {},
		},
		"return error when arc returns unauthorized": {
			setupARC: func(arc testservices.ARCFixture) {
				arc.WillAlwaysReturnStatus(http.StatusUnauthorized)
			},
		},
		"return error when arc returns forbidden": {
			setupARC: func(arc testservices.ARCFixture) {
				arc.WillAlwaysReturnStatus(http.StatusForbidden)
			},
		},
		"return error when arc returns not found": {
			setupARC: func(arc testservices.ARCFixture) {
				arc.WillAlwaysReturnStatus(http.StatusNotFound)
			},
		},
		"return error when arc returns internal server error": {
			setupARC: func(arc testservices.ARCFixture) {
				arc.WillAlwaysReturnStatus(http.StatusInternalServerError)
			},
		},
		"return error when trying to get merkle path for unknown tx": {
			setupARC: func(arc testservices.ARCFixture) {
				arc.IsUpAndRunning()
			},
		},
		"return error when arc returns invalid merkle path": {
			setupARC: func(arc testservices.ARCFixture) {
				arc.IsUpAndRunning()
				arc.WhenQueryingTx(txID).WillReturnTransactionWithMerklePathHex("invalid-merkle-path")
			},
		},
		"return error when arc return merkle path with invalid height": {
			setupARC: func(arc testservices.ARCFixture) {
				merklePath := sdk.MerklePath{
					BlockHeight: 2000,
					Path: [][]*sdk.PathElement{
						{
							{
								Offset: 0,
								Hash:   tx.TxID(),
								Txid:   to.Ptr(true),
							},
							{
								Offset: 1,
								Hash:   someSecondHash,
							},
						},
					},
				}

				arc.IsUpAndRunning()
				arc.WhenQueryingTx(txID).
					WillReturnTransactionWithMerklePath(merklePath).
					WillReturnTransactionOnHeight(2002)

			},
		},
		"return error when arc return merkle path without queried tx": {
			setupARC: func(arc testservices.ARCFixture) {
				merklePath := sdk.MerklePath{
					BlockHeight: 2000,
					Path: [][]*sdk.PathElement{
						{
							{
								Offset: 0,
								Hash:   someSecondHash,
							},
							{
								Offset: 1,
								Hash:   someSecondHash,
							},
						},
					},
				}

				arc.IsUpAndRunning()
				arc.WhenQueryingTx(txID).WillReturnTransactionWithMerklePath(merklePath)
			},
		},
	}
	for name, test := range arcErrorTestCases {
		t.Run(name, func(t *testing.T) {
			// given:
			given := arctestabilities.Given(t)

			// and:
			test.setupARC(given.ARC())

			// and:
			service := given.NewArcService()

			// when:
			res, err := service.MerklePath(t.Context(), txID)

			// then:
			assert.Error(t, err)
			assert.Nil(t, res)
		})
	}

	t.Run("return empty result if transaction is not mined yet", func(t *testing.T) {
		// given:
		given := arctestabilities.Given(t)

		// setup arc server
		given.ARC().IsUpAndRunning()

		// and:
		service := given.NewArcService()

		// and:
		given.ARC().WhenQueryingTx(txID).WillReturnTransactionWithoutMerklePath()

		// when:
		res, err := service.MerklePath(t.Context(), txID)

		// then:
		assert.NoError(t, err)
		assert.NotNil(t, res)
		require.Equal(t, wdk.MerklePathResult{
			Name:        "ARC",
			MerklePath:  nil,
			BlockHeader: nil,
		}, *res)
	})

	t.Run("return merkle path when arc return valid merkle path", func(t *testing.T) {
		// given:
		given := arctestabilities.Given(t)

		// setup arc server
		given.ARC().IsUpAndRunning()

		// and:
		service := given.NewArcService()

		merklePath := sdk.MerklePath{
			BlockHeight: 2000,
			Path: [][]*sdk.PathElement{
				{
					{
						Offset: 0,
						Hash:   tx.TxID(),
						Txid:   to.Ptr(true),
					},
					{
						Offset: 1,
						Hash:   someSecondHash,
					},
				},
			},
		}

		merkleRoot, err := merklePath.ComputeRootHex(nil)
		require.NoError(t, err, "failed to compute merkle root, wrong test setup")

		// and:
		given.ARC().WhenQueryingTx(txID).
			WillReturnTransactionWithMerklePath(merklePath)

		// when:
		res, err := service.MerklePath(t.Context(), txID)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, res)

		require.Equal(t, wdk.MerklePathResult{
			Name:       "ARC",
			MerklePath: &merklePath,
			BlockHeader: &wdk.MerklePathBlockHeader{
				Height:     2000,
				MerkleRoot: merkleRoot,
				Hash:       testservices.TestBlockHash,
			},
		}, *res)
	})
}
