package arc_test

import (
	"net/http"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arc"
	arctestabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arc/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestPostEFWithARCService(t *testing.T) {
	t.Run("broadcast single transaction", func(t *testing.T) {
		// given:
		given := arctestabilities.Given(t)

		// setup arc server
		given.ARC().IsUpAndRunning()

		// and:
		service := given.NewArcService()

		// and:
		tx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
		efTX, err := tx.EFHex()
		require.NoError(t, err)

		txID := tx.TxID().String()

		// when:
		res, err := service.PostEF(t.Context(), efTX, txID)

		// then:
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, wdk.PostedTxIDResultSuccess, res.Result)
		assert.Equal(t, txID, res.TxID)
		assert.Equal(t, given.ARC().TxInfoJSON(txID), res.Data)
		assert.Len(t, res.Notes, 1)
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
		efTX, err := tx.EFHex()
		require.NoError(t, err)

		txID := tx.TxID().String()

		// when:
		res, err := service.PostEF(t.Context(), efTX, txID)

		// then:
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, wdk.PostedTxIDResultSuccess, res.Result)
		assert.Equal(t, txID, res.TxID)
		assert.Equal(t, given.ARC().TxInfoJSON(txID), res.Data)
		assert.Len(t, res.Notes, 1)
	})

	invalidEFTestCases := map[string]struct {
		EF func(t testing.TB) (string, string)
	}{
		"return error on empty ef hex": {
			EF: func(t testing.TB) (string, string) {
				return "", "some-tx-id"
			},
		},
		"return error on invalid hex characters": {
			EF: func(t testing.TB) (string, string) {
				return "abc-not-hex", "some-tx-id"
			},
		},
	}
	for name, test := range invalidEFTestCases {
		t.Run(name, func(t *testing.T) {
			// given:
			given := arctestabilities.Given(t)
			txEF, txID := test.EF(t)

			// setup arc server
			given.ARC().IsUpAndRunning()

			// and:
			service := given.NewArcService()

			// when:
			res, err := service.PostEF(t.Context(), txEF, txID)

			// then:
			require.NoError(t, err)
			assert.NotNil(t, res)
			assert.Equal(t, wdk.PostedTxIDResultError, res.Result)
			assert.Error(t, res.Error)
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
			efHex, err := tx.EFHex()
			require.NoError(t, err)

			// when:
			res, err := service.PostEF(t.Context(), efHex, tx.TxID().String())

			// then:
			require.NoError(t, err)
			assert.NotNil(t, res)
			assert.Equal(t, wdk.PostedTxIDResultError, res.Result)
			assert.Error(t, res.Error)
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
			given.ARC().OnBroadcast().WillReturnNoBody()

			// and:
			service := given.NewArcService()

			// and:
			tx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
			// WithSender(txtestabilities.Alice).WithRecipient(txtestabilities.Alice).
			// WithInput(300).
			// WithP2PKHOutput(299).
			// TX()
			txID := tx.TxID().String()
			efHex, err := tx.EFHex()
			require.NoError(t, err)

			// and:
			test.setupARCQueryTx(given.ARC().WhenQueryingTx(txID))

			// when:
			res, err := service.PostEF(t.Context(), efHex, txID)

			// then:
			require.NoError(t, err)
			require.NotNil(t, res)

			assert.Equal(t, wdk.PostedTxIDResultError, res.Result)
			assert.Empty(t, res.Data)
			assert.Error(t, res.Error)
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
			require.Error(t, err)
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
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, arc.ServiceName, res.Name)
		assert.Nil(t, res.MerklePath)
		assert.Nil(t, res.BlockHeader)
		require.Len(t, res.Notes, 1)
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
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, arc.ServiceName, res.Name)
		assert.Equal(t, merklePath, *res.MerklePath)
		assert.Equal(t, wdk.MerklePathBlockHeader{
			Height:     2000,
			MerkleRoot: merkleRoot,
			Hash:       testservices.TestBlockHash,
		}, *res.BlockHeader)
		assert.Len(t, res.Notes, 1)
	})
}
