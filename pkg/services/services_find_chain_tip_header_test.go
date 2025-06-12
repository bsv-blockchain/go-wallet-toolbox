package services_test

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/stretchr/testify/require"
)

func TestFindChainTipHeader(t *testing.T) {
	t.Run("return a single block header after call to the whats on chain service", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		blocks := given.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()

		// and:
		expectedBlock, err := blocks[0].ConvertToChainBlockHeader()
		require.NoError(t, err)
		require.NotNil(t, expectedBlock)

		// and:
		service := given.Services().WithDefaultConfig()

		// when:
		actualBlock, err := service.FindChainTipHeader(t.Context())

		// then:
		require.NoError(t, err)
		require.Equal(t, expectedBlock, actualBlock)
	})

	t.Run("return an error when all block header services responds with internal server error", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.WhatsOnChain().WillRespondWithInternalFailure()

		// and:
		service := given.Services().WithDefaultConfig()

		// when:
		actualBlock, err := service.FindChainTipHeader(t.Context())

		// then:
		require.Error(t, err)
		require.Nil(t, actualBlock)
	})

	t.Run("return an error when all block header services are unreachable", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		target := given.WhatsOnChain().WillBeUnreachable()

		// and:
		service := given.Services().WithDefaultConfig()

		// when:
		actualBlock, err := service.FindChainTipHeader(t.Context())

		// then:
		require.ErrorIs(t, err, target)
		require.Nil(t, actualBlock)
	})

	t.Run("return an error when all block header services return an empty header blocks response", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.WhatsOnChain().OnTipBlockHeaderWillRespondWithEmptyList()

		// and:
		service := given.Services().WithDefaultConfig()

		// when:
		actualBlock, err := service.FindChainTipHeader(t.Context())

		// then:
		require.Error(t, err)
		require.Nil(t, actualBlock)
	})
}
