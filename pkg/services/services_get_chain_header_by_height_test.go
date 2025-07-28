package services_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetChainHeaderByHeight_PositivePath(t *testing.T) {
	// given:
	const height = 1024

	given := testservices.GivenServices(t)
	svc := given.Services().WithDefaultConfig()

	// and:
	bhs := given.BHS()
	bhs.IsUpAndRunning()
	first := bhs.DefaultHeaderByHeightResponse()[0]

	expectedHeader := &wdk.ChainBaseBlockHeader{
		Version:      uint32(first.Version),
		PreviousHash: first.PreviousBlock,
		MerkleRoot:   first.MerkleRoot,
		Time:         first.Timestamp,
		Bits:         first.DifficultyTarget,
		Nonce:        first.Nonce,
	}

	// when:
	actualHeader, err := svc.GetChainHeaderByHeight(t.Context(), height)

	// then:
	require.NoError(t, err)
	require.Equal(t, expectedHeader, actualHeader)
}

func TestGetChainHeaderByHeight_NegativePaths(t *testing.T) {
	t.Run("return error when all services are unreachable", func(t *testing.T) {
		// given:
		const height = 1024

		given := testservices.GivenServices(t)
		expectedSubstr := given.BHS().WillBeUnreachable().Error()

		// and:
		services := given.Services().WithDefaultConfig()

		// when:
		header, err := services.GetChainHeaderByHeight(t.Context(), height)

		// then:
		isNotMockTransportResponderError(t, err)

		assert.ErrorContains(t, err, expectedSubstr)
		assert.Nil(t, header)
	})

	t.Run("return an error when all block header services respond with internal server error", func(t *testing.T) {
		// given:
		const height = 1024

		given := testservices.GivenServices(t)
		given.BHS().WillRespondWithInternalFailure()

		// and:
		services := given.Services().WithDefaultConfig()

		// when:
		response, err := services.GetChainHeaderByHeight(t.Context(), height)

		// then:
		isNotMockTransportResponderError(t, err)

		assert.NotNil(t, err)
		assert.Nil(t, response)
	})

	t.Run("return an error when all block header services return an empty header blocks response", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.BHS().WillRespondWithEmptyHeaderByHeightResponse()

		// and:
		service := given.Services().WithDefaultConfig()

		// when:
		actualBlock, err := service.GetChainHeaderByHeight(t.Context(), 0) // Assuming height 0 for empty response scenario

		// then:
		isNotMockTransportResponderError(t, err)
		require.Error(t, err)
		require.Nil(t, actualBlock)
	})
}
