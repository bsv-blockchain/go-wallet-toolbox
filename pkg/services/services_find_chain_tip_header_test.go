package services_test

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails"
	"github.com/stretchr/testify/require"
)

func TestFindChainTipHeader_Bitails(t *testing.T) {
	t.Run("return a single block header after call to the bitails service", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		headerHex := testservices.TestFakeHeaderBinary
		raw, _ := hex.DecodeString(headerHex)
		hash := chainhash.DoubleHashH(raw).String()
		height := testservices.TestBlockHeight

		given.Bitails().WillReturnLatestBlock(hash, uint32(height))
		given.Bitails().WillReturnBlockHeader(hash, headerHex)

		expectedBlock, err := bitails.ConvertHeader(raw, uint32(height))
		require.NoError(t, err)

		service := given.Services().WithDefaultConfig()

		// when:
		actualBlock, err := service.FindChainTipHeader(t.Context())

		// then:
		isNotMockTransportResponderError(t, err)
		require.NoError(t, err)
		require.Equal(t, expectedBlock, actualBlock)
	})

	t.Run("return longest tip block header from bitails service when other providers fail", func(t *testing.T) {
		// given:
		const expectedBlockHeight = 2048
		given := testservices.GivenServices(t)

		given.Bitails().WillReturnLatestBlock(testservices.TestBlockHash, uint32(expectedBlockHeight))
		given.Bitails().WillReturnBlockHeader(testservices.TestBlockHash, testservices.TestFakeHeaderBinary)
		given.BHS().WillRespondWithInternalFailure()
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

	t.Run("return an error when all block header services respond with internal server error", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.BHS().WillRespondWithInternalFailure()
		given.WhatsOnChain().WillRespondWithInternalFailure()
		given.Bitails().WillReturnInternalError()

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
		target3 := given.Bitails().WillBeUnreachable()

		// and:
		service := given.Services().WithDefaultConfig()

		// when:
		actualBlock, err := service.FindChainTipHeader(t.Context())

		// then:
		isNotMockTransportResponderError(t, err)

		require.ErrorIs(t, err, target1)
		require.ErrorIs(t, err, target2)
		require.ErrorIs(t, err, target3)
		require.Nil(t, actualBlock)
	})

	t.Run("return an error when all block header services return an empty header blocks response", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.BHS().WillRespondWithEmptyLongestTipBlockHeader()
		given.WhatsOnChain().OnTipBlockHeaderWillRespondWithEmptyList()
		given.Bitails().WillReturnLatestBlock("", 0)

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
