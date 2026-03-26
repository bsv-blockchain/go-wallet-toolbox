package services_test

import (
	"net/http"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestPostFromBEEF(t *testing.T) {
	t.Run("successfully post from BEEF with single tx IDs", func(t *testing.T) {
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

		services := given.Services().Config(testservices.WithEnabledBitails(true)).New()

		// when:
		response, err := services.PostFromBEEF(t.Context(), beef, txids)

		// then:
		require.NoError(t, err)
		assert.NotEmpty(t, response)

		slices.ForEach(response, func(item *wdk.PostFromBEEFServiceResult) {
			assert.NotEmpty(t, item.Name)
			require.NoError(t, item.Error)
			if assert.NotNil(t, item.PostedBEEFResult) {
				result := item.PostedBEEFResult
				assert.Lenf(t, result.TxIDResults, len(txids), "service %s returned unexpected number of results", item.Name)
			}
		})
	})

	t.Run("successfully post from BEEF with multiple tx IDs", func(t *testing.T) {
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

		services := given.Services().Config(testservices.WithEnabledBitails(true)).New()

		// when:
		response, err := services.PostFromBEEF(t.Context(), beef, txids)

		// then:
		require.NoError(t, err)
		assert.NotEmpty(t, response)

		// and then: grouped by service and verify each service handled both txIDs
		resultsByService := groupResultsByService(response)
		for serviceName, results := range resultsByService {
			assert.Len(t, results, 2, "service %s should have 2 results (one per txID)", serviceName)
		}
	})
}

func TestPostFromBEEF_BroadcastFailures(t *testing.T) {
	// NOTE: We only broadcast the childTx (the unmined tx in the BEEF).
	// The parentTx is just a source input for childTx and doesn't need to be broadcast separately.
	parentTx := txtestabilities.GivenTX().
		WithSender(txtestabilities.Alice).WithRecipient(txtestabilities.Alice).
		WithInput(100).
		WithP2PKHOutput(99).
		TX()

	childTx := txtestabilities.GivenTX().WithInputFromUTXO(parentTx, 0).WithP2PKHOutput(98).TX()
	childTxID := childTx.TxID().String()

	beef, err := sdk.NewBeefFromTransaction(childTx)
	require.NoError(t, err)

	// Only the child tx needs to be broadcast - parent is just a source tx (input)
	txids := []string{childTxID}

	t.Run("WoC returns error, rest return success", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.ARC().IsUpAndRunning()

		given.WhatsOnChain().WillRespondWithBroadcast(http.StatusInternalServerError, "WoC internal error")
		given.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{})

		given.Bitails().WillReturnSuccessAndTxInfo(childTxID, "mocked-block-hash", 99999)

		services := given.Services().Config(testservices.WithEnabledBitails(true)).New()

		// when:
		response, err := services.PostFromBEEF(t.Context(), beef, txids)

		// then:
		require.NoError(t, err)
		require.NotEmpty(t, response)

		resultsByService := groupResultsByService(response)

		// and then: WoC should have errors
		for _, res := range resultsByService["WhatsOnChain"] {
			assertSingleResultHasError(t, res)
		}

		// and then: ARC and Bitails should succeed
		for _, res := range resultsByService["ARC"] {
			assertSingleResultHasSuccess(t, res)
		}
		for _, res := range resultsByService["Bitails"] {
			assertSingleResultHasSuccess(t, res)
		}
	})

	t.Run("ARC returns error, rest return success", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		given.WhatsOnChain().WillAlwaysReturnPostBEEFSuccess(childTxID)
		given.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{})
		given.ARC().WillAlwaysReturnStatus(http.StatusInternalServerError)

		given.Bitails().WillReturnSuccessAndTxInfo(childTxID, "mocked-block-hash", 99999)

		services := given.Services().Config(testservices.WithEnabledBitails(true)).New()

		// when:
		response, err := services.PostFromBEEF(t.Context(), beef, txids)

		// then:
		require.NoError(t, err)
		require.NotEmpty(t, response)

		resultsByService := groupResultsByService(response)

		// and then: ARC should have errors
		for _, res := range resultsByService["ARC"] {
			assertSingleResultHasError(t, res)
		}

		// and then: WoC and Bitails should succeed
		for _, res := range resultsByService["WhatsOnChain"] {
			assertSingleResultHasSuccess(t, res)
		}
		for _, res := range resultsByService["Bitails"] {
			assertSingleResultHasSuccess(t, res)
		}
	})

	t.Run("All services return errors", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		given.WhatsOnChain().WillRespondWithBroadcast(http.StatusInternalServerError, "WoC internal error")
		given.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{})
		given.ARC().WillAlwaysReturnStatus(http.StatusInternalServerError)
		given.Bitails().OnBroadcast().WillReturnHttpError(http.StatusInternalServerError)

		services := given.Services().Config(testservices.WithEnabledBitails(true)).New()

		// when:
		response, err := services.PostFromBEEF(t.Context(), beef, txids)

		// then:
		require.NoError(t, err)
		require.NotEmpty(t, response)

		for _, res := range response {
			assertSingleResultHasError(t, res)
		}
	})
}

func groupResultsByService(results wdk.PostFromBeefResult) map[string][]*wdk.PostFromBEEFServiceResult {
	grouped := make(map[string][]*wdk.PostFromBEEFServiceResult)
	for _, res := range results {
		grouped[res.Name] = append(grouped[res.Name], res)
	}
	return grouped
}

func assertSingleResultHasError(t *testing.T, res *wdk.PostFromBEEFServiceResult) {
	t.Helper()

	require.NoError(t, res.Error, "unexpected service-level error for %s", res.Name)
	require.NotNil(t, res.PostedBEEFResult, "expected result for service %s", res.Name)
	require.Len(t, res.PostedBEEFResult.TxIDResults, 1)
	assert.Equal(t, wdk.PostedTxIDResultError, res.PostedBEEFResult.TxIDResults[0].Result, "expected error result for service %s", res.Name)
}

func assertSingleResultHasSuccess(t *testing.T, res *wdk.PostFromBEEFServiceResult) {
	t.Helper()

	require.NoError(t, res.Error, "unexpected service-level error for %s: %w", res.Name, res.Error)
	require.NotNil(t, res.PostedBEEFResult, "expected result for service %s", res.Name)
	require.Len(t, res.PostedBEEFResult.TxIDResults, 1)
	assert.Equal(t, wdk.PostedTxIDResultSuccess, res.PostedBEEFResult.TxIDResults[0].Result,
		"expected success result for service %s", res.Name)
	require.NotNil(t, res.PostedBEEFResult.TxIDResults[0].Notes)
}
