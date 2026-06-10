package services_test

import (
	"net/http"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const arcadeBroadcastURL = defs.ArcadeURL + "/tx"

func givenArcadeBroadcastSucceeds(given testservices.ServicesFixture, txID string) {
	given.Transport().RegisterResponder(http.MethodPost, arcadeBroadcastURL,
		httpmock.NewJsonResponderOrPanic(http.StatusOK, map[string]any{
			"txid":     txID,
			"txStatus": "SEEN_ON_NETWORK",
		}),
	)
}

func givenArcadeRejectsTransaction(given testservices.ServicesFixture) {
	given.Transport().RegisterResponder(http.MethodPost, arcadeBroadcastURL,
		httpmock.NewJsonResponderOrPanic(http.StatusBadRequest, map[string]any{
			"error": "mocked arcade rejection",
		}),
	)
}

func givenGorillaPoolARCIsFailing(given testservices.ServicesFixture) {
	given.Transport().RegisterResponder(http.MethodPost, defs.GorillaPoolArcURL+"/v1/tx",
		httpmock.NewJsonResponderOrPanic(http.StatusInternalServerError, map[string]any{
			"error": "mocked gorillapool outage",
		}),
	)
}

func TestPostFromBEEF(t *testing.T) {
	t.Run("successfully post from BEEF with single tx ID broadcasts through Arcade only", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		// and:
		tx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
		beef, err := sdk.NewBeefFromTransaction(tx)
		require.NoError(t, err)

		txID := tx.TxID().String()
		txids := []string{txID}

		// and: Arcade accepts the broadcast; failover services are configured but must not be used
		givenArcadeBroadcastSucceeds(given, txID)
		given.ARC().IsUpAndRunning()

		services := given.Services().Config(testservices.WithEnabledBitails(true)).New()

		// when:
		response, err := services.PostFromBEEF(t.Context(), beef, txids)

		// then: exactly one result - Arcade is the sole default broadcast path
		require.NoError(t, err)
		require.Len(t, response, 1)
		assert.Equal(t, defs.ArcadeServiceName, response[0].Name)
		assertSingleResultHasSuccess(t, response[0])
	})

	t.Run("successfully post from BEEF with multiple tx IDs", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

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

		// and:
		givenArcadeBroadcastSucceeds(given, childTxID)

		services := given.Services().Config(testservices.WithEnabledBitails(true)).New()

		// when:
		response, err := services.PostFromBEEF(t.Context(), beef, txids)

		// then: one Arcade result per txID
		require.NoError(t, err)
		require.Len(t, response, len(txids))

		resultsByService := groupResultsByService(response)
		require.Len(t, resultsByService[defs.ArcadeServiceName], 2, "Arcade should have 2 results (one per txID)")
		for _, res := range resultsByService[defs.ArcadeServiceName] {
			assertSingleResultHasSuccess(t, res)
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

	t.Run("Arcade unreachable, fails over to ARC", func(t *testing.T) {
		// given: no Arcade responder is registered - the broadcast fails on transport
		given := testservices.GivenServices(t)
		given.ARC().IsUpAndRunning()

		services := given.Services().Config(testservices.WithEnabledBitails(true)).New()

		// when:
		response, err := services.PostFromBEEF(t.Context(), beef, txids)

		// then: the Arcade transport failure and the winning ARC result are both reported
		require.NoError(t, err)
		require.Len(t, response, 2)

		assert.Equal(t, defs.ArcadeServiceName, response[0].Name)
		require.Error(t, response[0].Error)
		assert.Nil(t, response[0].PostedBEEFResult)

		assert.Equal(t, defs.ArcServiceName, response[1].Name)
		assertSingleResultHasSuccess(t, response[1])
	})

	t.Run("Arcade rejects the tx, no failover happens", func(t *testing.T) {
		// given: a tx-level rejection from Arcade is a final verdict, not a service failure
		given := testservices.GivenServices(t)
		givenArcadeRejectsTransaction(given)
		given.ARC().IsUpAndRunning()

		services := given.Services().Config(testservices.WithEnabledBitails(true)).New()

		// when:
		response, err := services.PostFromBEEF(t.Context(), beef, txids)

		// then: only the Arcade rejection is reported - failover services were not consulted
		require.NoError(t, err)
		require.Len(t, response, 1)
		assert.Equal(t, defs.ArcadeServiceName, response[0].Name)
		assertSingleResultHasError(t, response[0])
	})

	t.Run("Arcade unreachable and ARC returns error", func(t *testing.T) {
		// given: no Arcade responder (transport failure), TAAL ARC answers HTTP 500
		// and GorillaPool ARC is down too - both fold the failure into the result
		given := testservices.GivenServices(t)
		given.ARC().WillAlwaysReturnStatus(http.StatusInternalServerError)
		givenGorillaPoolARCIsFailing(given)

		// and: WhatsOnChain accepts the broadcast
		given.WhatsOnChain().WillAlwaysReturnPostBEEFSuccess(childTxID)

		services := given.Services().Config(testservices.WithEnabledBitails(true)).New()

		// when:
		response, err := services.PostFromBEEF(t.Context(), beef, txids)

		// then: every failed service is reported as an error result in chain order,
		// and the chain continues past the folded ARC errors until the first
		// acceptance (WhatsOnChain) - Bitails is never consulted
		require.NoError(t, err)
		require.Len(t, response, 4)

		failedServices := []string{defs.ArcadeServiceName, defs.ArcServiceName, defs.ArcGorillaPoolServiceName}
		for i, name := range failedServices {
			assert.Equal(t, name, response[i].Name)
			require.Error(t, response[i].Error, "expected transport-level error for %s", name)
			assert.Nil(t, response[i].PostedBEEFResult, "no posted result expected for failed service %s", name)
		}

		assert.Equal(t, defs.WhatsOnChainServiceName, response[3].Name)
		assertSingleResultHasSuccess(t, response[3])
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
