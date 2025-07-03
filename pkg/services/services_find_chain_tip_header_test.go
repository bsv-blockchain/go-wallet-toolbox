package services_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/require"
)

func TestFindChainTipHeader(t *testing.T) {
	t.Run("return longest tip block header from block header service when whats on chain service responds with internal server error", func(t *testing.T) {
		// given:
		const expectedBlockHeight = 1024
		given := testservices.GivenServices(t)

		given.BHS().OnLongestTipBlockHeaderResponseWith(testservices.WithLongestChainTipHeight(1024))
		given.BHS().IsUpAndRunning()
		given.WhatsOnChain().WillRespondWithInternalFailure()

		// and:
		service := given.Services().WithDefaultConfig()

		// when:
		actualBlock, err := service.FindChainTipHeader(t.Context())

		// then:
		require.Nil(t, err)
		require.NotEmpty(t, actualBlock)
		require.EqualValues(t, expectedBlockHeight, actualBlock.Height)
	})

	t.Run("return a single block header after call to the whats on chain service", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()

		// and:
		expectedBlock := newTestChainBlockHeader(t)

		// and:
		service := given.Services().WithDefaultConfig()

		// when:
		actualBlock, err := service.FindChainTipHeader(t.Context())

		// then:
		isNotMockTransportResponderError(t, err)

		require.NoError(t, err)
		require.Equal(t, expectedBlock, actualBlock)
	})

	t.Run("return an error when all block header services responds with internal server error", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.BHS().WillRespondWithInternalFailure()
		given.WhatsOnChain().WillRespondWithInternalFailure()

		// and:
		service := given.Services().WithDefaultConfig()

		// when:
		actualBlock, err := service.FindChainTipHeader(t.Context())

		// then:
		isNotMockTransportResponderError(t, err)

		require.Error(t, err)
		require.Nil(t, actualBlock)
	})

	t.Run("return an error when all block header services are unreachable", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		target1 := given.BHS().WillBeUnreachable()
		target2 := given.WhatsOnChain().WillBeUnreachable()

		// and:
		service := given.Services().WithDefaultConfig()

		// when:
		actualBlock, err := service.FindChainTipHeader(t.Context())

		// then:
		isNotMockTransportResponderError(t, err)

		require.ErrorIs(t, err, target1)
		require.ErrorIs(t, err, target2)
		require.Nil(t, actualBlock)
	})

	t.Run("return an error when all block header services return an empty header blocks response", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.BHS().WillRespondWithEmptyLongestTipBlockHeader()
		given.WhatsOnChain().OnTipBlockHeaderWillRespondWithEmptyList()

		// and:
		service := given.Services().WithDefaultConfig()

		// when:
		actualBlock, err := service.FindChainTipHeader(t.Context())

		// then:
		isNotMockTransportResponderError(t, err)
		require.Error(t, err)
		require.Nil(t, actualBlock)
	})
}

func isNotMockTransportResponderError(t *testing.T, err error) {
	t.Helper()
	require.NotErrorIs(t, err, errors.New("no responder found"))
}

func newTestChainBlockHeader(t *testing.T) *wdk.ChainBlockHeader {
	t.Helper()

	bits, err := strconv.ParseUint(testservices.TestBlockBits, 16, 64)
	require.NotZero(t, bits)
	require.NoError(t, err)

	return &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
			Version:      testservices.TestBlockVersion,
			PreviousHash: testservices.TestBlockPreviousBlockHash,
			MerkleRoot:   testservices.TestBlockMerkleRoot,
			Time:         testservices.TestBlockTime,
			Bits:         bits,
			Nonce:        testservices.TestBlockNonce,
		},
		Height: testservices.TestBlockHeight,
		Hash:   testservices.TestBlockHash,
	}
}
