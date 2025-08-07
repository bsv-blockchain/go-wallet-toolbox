package services_test

import (
	"net/http"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostBEEF(t *testing.T) {
	t.Run("successfully post BEEF with single tx IDs", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.ARC().IsUpAndRunning()

		// and:
		tx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
		beef, err := sdk.NewBeefFromTransaction(tx)
		require.NoError(t, err)

		txID := tx.TxID().String()
		txids := []string{txID}

		given.WhatsOnChain().WillAlwaysReturnPostBEEFSuccess(txID)
		given.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{})
		given.Bitails().WillReturnSuccessAndTxInfo(txID, "mocked-block-hash", 99999)

		services := given.Services().WithDefaultConfig()

		// when:
		response, err := services.PostBEEF(t.Context(), beef, txids)

		// then:
		assert.NoError(t, err)
		assert.NotEmpty(t, response)

		slices.ForEach(response, func(item *wdk.PostBEEFServiceResult) {
			assert.NotEmpty(t, item.Name)
			assert.NoError(t, item.Error)
			if assert.NotNil(t, item.PostedBEEFResult) {
				result := item.PostedBEEFResult
				assert.Lenf(t, result.TxIDResults, len(txids), "service %s returned unexpected number of results", item.Name)
			}
		})
	})

	t.Run("successfully post BEEF with multiple tx IDs", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.ARC().IsUpAndRunning()

		parentTx := txtestabilities.GivenTX().
			WithSender(txtestabilities.Alice).WithRecipient(txtestabilities.Alice).
			WithInput(100).
			WithP2PKHOutput(99).
			TX()
		parentTxID := parentTx.TxID().String()

		childTx := txtestabilities.GivenTX().WithInputFromUTXO(parentTx, 0).WithP2PKHOutput(98).TX()
		childTxID := childTx.TxID().String()
		beef, err := sdk.NewBeefFromTransaction(childTx)
		require.NoError(t, err)

		txids := []string{parentTxID, childTxID}

		given.WhatsOnChain().WillAlwaysReturnPostBEEFSuccess(parentTxID, childTxID)
		given.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{})

		given.Bitails().WillReturnSuccessAndTxInfo(parentTxID, "mocked-block-hash", 99999)
		given.Bitails().WillReturnSuccessAndTxInfo(childTxID, "mocked-block-hash", 99999)

		services := given.Services().WithDefaultConfig()

		// when:
		response, err := services.PostBEEF(t.Context(), beef, txids)

		// then:
		assert.NoError(t, err)
		assert.NotEmpty(t, response)

		slices.ForEach(response, func(item *wdk.PostBEEFServiceResult) {
			assert.NotEmpty(t, item.Name)
			assert.NoError(t, item.Error)
			if assert.NotNil(t, item.PostedBEEFResult) {
				result := item.PostedBEEFResult
				assert.Lenf(t, result.TxIDResults, len(txids), "service %s returned unexpected number of results", item.Name)
			}
		})
	})
}

func TestPostBEEF_BroadcastFailures(t *testing.T) {
	parentTx := txtestabilities.GivenTX().
		WithSender(txtestabilities.Alice).WithRecipient(txtestabilities.Alice).
		WithInput(100).
		WithP2PKHOutput(99).
		TX()
	parentTxID := parentTx.TxID().String()

	childTx := txtestabilities.GivenTX().WithInputFromUTXO(parentTx, 0).WithP2PKHOutput(98).TX()
	childTxID := childTx.TxID().String()

	beef, err := sdk.NewBeefFromTransaction(childTx)
	require.NoError(t, err)

	txids := []string{parentTxID, childTxID}

	t.Run("WoC returns error, rest return success", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.ARC().IsUpAndRunning()

		for range txids {
			given.WhatsOnChain().WillRespondWithBroadcast(http.StatusInternalServerError, "WoC internal error")
		}
		given.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{})

		for _, txid := range txids {
			given.Bitails().WillReturnSuccessAndTxInfo(txid, "mocked-block-hash", 99999)
		}

		services := given.Services().WithDefaultConfig()

		// when:
		response, err := services.PostBEEF(t.Context(), beef, txids)

		// then:
		require.NoError(t, err)
		require.NotEmpty(t, response)

		for _, res := range response {
			switch res.Name {
			case "WhatsOnChain":
				assertWoCErrorResult(t, res, txids)
			default:
				assertServiceSuccess(t, res, txids)
			}
		}
	})

	t.Run("ARC returns error, rest return success", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		given.WhatsOnChain().WillAlwaysReturnPostBEEFSuccess(txids...)
		given.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{})
		given.ARC().WillAlwaysReturnStatus(http.StatusInternalServerError)

		for _, txid := range txids {
			given.Bitails().WillReturnSuccessAndTxInfo(txid, "mocked-block-hash", 99999)
		}

		services := given.Services().WithDefaultConfig()

		// when:
		response, err := services.PostBEEF(t.Context(), beef, txids)

		// then:
		require.NoError(t, err)
		require.NotEmpty(t, response)

		for _, res := range response {
			switch res.Name {
			case "ARC":
				assertArcErrorResult(t, res)
			default:
				assertServiceSuccess(t, res, txids)
			}
		}
	})

	t.Run("All services return errors", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		for range txids {
			given.WhatsOnChain().WillRespondWithBroadcast(http.StatusInternalServerError, "WoC internal error")
		}
		given.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{})
		given.ARC().WillAlwaysReturnStatus(http.StatusInternalServerError)
		given.Bitails().OnBroadcast().WillReturnHttpError(http.StatusInternalServerError)

		services := given.Services().WithDefaultConfig()

		// when:
		response, err := services.PostBEEF(t.Context(), beef, txids)

		// then:
		require.NoError(t, err)
		require.NotEmpty(t, response)

		for _, res := range response {
			switch res.Name {
			case "WhatsOnChain":
				assertWoCErrorResult(t, res, txids)
			case "ARC":
				assertArcErrorResult(t, res)
			case "Bitails":
				assertBitailsErrorResult(t, res, txids)
			default:
				t.Fatalf("Unexpected service name: %s", res.Name)
			}
		}
	})
}

func assertWoCErrorResult(t *testing.T, res *wdk.PostBEEFServiceResult, txids []string) {
	require.NoError(t, res.Error)
	require.NotNil(t, res.PostedBEEFResult)
	require.Len(t, res.PostedBEEFResult.TxIDResults, len(txids))
	for _, txResult := range res.PostedBEEFResult.TxIDResults {
		require.Equal(t, wdk.PostedTxIDResultError, txResult.Result)
		require.NotEmpty(t, txResult.Notes)
	}
}

func assertArcErrorResult(t *testing.T, res *wdk.PostBEEFServiceResult) {
	require.Error(t, res.Error)
	require.Nil(t, res.PostedBEEFResult)
}

func assertBitailsErrorResult(t *testing.T, res *wdk.PostBEEFServiceResult, txids []string) {
	require.NoError(t, res.Error)
	require.NotNil(t, res.PostedBEEFResult)
	require.Len(t, res.PostedBEEFResult.TxIDResults, len(txids))
	for _, txResult := range res.PostedBEEFResult.TxIDResults {
		require.Equal(t, wdk.PostedTxIDResultError, txResult.Result)
		require.NotEmpty(t, txResult.Notes)
	}
}

func assertServiceSuccess(t *testing.T, res *wdk.PostBEEFServiceResult, txids []string) {
	require.NoError(t, res.Error)
	require.NotNil(t, res.PostedBEEFResult)
	require.Len(t, res.PostedBEEFResult.TxIDResults, len(txids))
}
