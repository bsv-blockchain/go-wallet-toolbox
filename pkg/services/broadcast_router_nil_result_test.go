package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/circuitbreaker"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// nilResultResponse breaks the broadcastTarget contract: nil result, nil error.
func nilResultResponse() func() (*wdk.PostedTxID, error) {
	return func() (*wdk.PostedTxID, error) { return nil, nil }
}

func TestBroadcastRouterPrimaryNilResultFailsOverAndTripsBreaker(t *testing.T) {
	// given: failure threshold 1 and a primary that breaks the broadcastTarget
	// contract (nil result, nil error)
	given := newRouterFixture(t, 1)
	given.primary.responses = []func() (*wdk.PostedTxID, error){nilResultResponse()}
	failover := &fakeTarget{name: testFailoverName, responses: []func() (*wdk.PostedTxID, error){
		successResponse(),
		successResponse(),
	}}
	given.withFailovers(failover)

	// when:
	results := given.broadcast()

	// then: the primary error result is followed by the winning failover success
	require.Len(t, results, 2)
	assert.Equal(t, "Arcade", results[0].Name)
	require.ErrorContains(t, results[0].Error, "service returned no result")
	assert.Nil(t, results[0].PostedBEEFResult)
	assert.Equal(t, testFailoverName, results[1].Name)
	require.NoError(t, results[1].Error)
	require.NotNil(t, results[1].PostedBEEFResult)

	// and: the contract violation counted as a breaker failure (threshold 1 - open)
	require.Equal(t, circuitbreaker.StateOpen, given.breaker.State())

	// when: broadcasting again within the probe window
	results = given.broadcast()

	// then: the primary is skipped entirely - only the failover responds
	require.Len(t, results, 1)
	assert.Equal(t, testFailoverName, results[0].Name)
	require.NoError(t, results[0].Error)
	assert.Equal(t, 1, given.primary.calls)
}

func TestBroadcastRouterFailoverNilResultContinuesChain(t *testing.T) {
	// given: the primary fails on transport and the first failover breaks the
	// broadcastTarget contract (nil result, nil error)
	given := newRouterFixture(t, 3)
	given.primary.responses = []func() (*wdk.PostedTxID, error){transportErrorResponse()}
	first := &fakeTarget{name: testFailoverName, responses: []func() (*wdk.PostedTxID, error){nilResultResponse()}}
	second := &fakeTarget{name: "WhatsOnChain", responses: []func() (*wdk.PostedTxID, error){successResponse()}}
	given.withFailovers(first, second)

	// when:
	results := given.broadcast()

	// then: the nil-result target is reported as an error and the chain went on
	require.Len(t, results, 3)
	assert.Equal(t, "Arcade", results[0].Name)
	require.Error(t, results[0].Error)
	assert.Equal(t, testFailoverName, results[1].Name)
	require.ErrorContains(t, results[1].Error, "service returned no result")
	assert.Nil(t, results[1].PostedBEEFResult)
	assert.Equal(t, "WhatsOnChain", results[2].Name)
	require.NoError(t, results[2].Error)
	require.NotNil(t, results[2].PostedBEEFResult)

	// and: the chain stopped at the first success
	assert.Equal(t, 1, first.calls)
	assert.Equal(t, 1, second.calls)
}

func TestBroadcastRouterPrimaryNilResultWithoutFailoversReturnsErrorResult(t *testing.T) {
	// given: a primary that breaks the broadcastTarget contract and no failovers
	given := newRouterFixture(t, 3)
	given.primary.responses = []func() (*wdk.PostedTxID, error){nilResultResponse()}

	// when:
	results := given.broadcast()

	// then: an error result instead of an empty slice or a panic
	require.Len(t, results, 1)
	assert.Equal(t, "Arcade", results[0].Name)
	require.ErrorContains(t, results[0].Error, "service returned no result")
	assert.Nil(t, results[0].PostedBEEFResult)
}
