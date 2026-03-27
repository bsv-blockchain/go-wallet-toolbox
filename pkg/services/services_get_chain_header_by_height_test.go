package services_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestGetChainHeaderByHeight_AtLeastOneChainServiceIsResponsive(t *testing.T) {
	t.Run("return chain base block header when only Bitails is responsive", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		svc := given.Services().Config(testservices.WithEnabledBitails(true)).New()

		// and:
		given.BHS().WillRespondWithInternalFailure()
		given.WhatsOnChain().WillRespondWithInternalFailure()
		given.Bitails().WillRespondWithBlockByHeight()

		bits, err := strconv.ParseUint(testservices.TestBlockBits, 16, 32)
		require.NoError(t, err)

		expectedHeader := &wdk.ChainBlockHeader{
			ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
				MerkleRoot:   testservices.TestBlockMerkleRoot,
				Version:      testservices.TestBlockVersion,
				PreviousHash: testservices.TestBlockPreviousBlockHash,
				Time:         uint32(testservices.TestBlockTime),
				Bits:         uint32(bits),
				Nonce:        testservices.TestBlockNonce,
			},
			Hash: testservices.TestBlockHash,
		}

		// when:
		actualHeader, err := svc.ChainHeaderByHeight(t.Context(), testservices.TestBlockHeight)

		// then:
		require.NoError(t, err)
		require.Equal(t, expectedHeader, actualHeader)
	})

	t.Run("return chain base block header when only WOC service is responsive", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		svc := given.Services().Config(testservices.WithEnabledBitails(true)).New()

		// and:
		given.BHS().WillRespondWithInternalFailure()
		given.Bitails().WillRespondWithInternalFailure()
		given.WhatsOnChain().WillRespondWithBlockHeaderByHeight(http.StatusOK, testservices.TestBlockHeight, testservices.TestBlockMerkleRoot)

		bits, err := strconv.ParseUint(testservices.TestBlockBits, 16, 32)
		require.NoError(t, err)

		expectedHeader := &wdk.ChainBlockHeader{
			ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
				MerkleRoot:   testservices.TestBlockMerkleRoot,
				Version:      testservices.TestBlockVersion,
				PreviousHash: testservices.TestBlockPreviousBlockHash,
				Time:         uint32(testservices.TestBlockTime),
				Bits:         uint32(bits),
				Nonce:        testservices.TestBlockNonce,
			},
			Hash:   testservices.TestBlockHash,
			Height: testservices.TestBlockHeight,
		}

		// when:
		actualHeader, err := svc.ChainHeaderByHeight(t.Context(), testservices.TestBlockHeight)

		// then:
		require.NoError(t, err)
		require.Equal(t, expectedHeader, actualHeader)
	})

	t.Run("return chain base block header when only BHS service is responsive", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		svc := given.Services().Config(testservices.WithEnabledBitails(true), testservices.WithEnabledBHS(true)).New()

		// and:
		given.WhatsOnChain().WillRespondWithInternalFailure()
		given.Bitails().WillRespondWithInternalFailure()
		first := given.BHS().IsUpAndRunning().DefaultHeaderByHeightResponse()

		expectedHeader := &wdk.ChainBlockHeader{
			ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
				Version:      uint32(first.Version), //nolint:gosec // block header version is always small positive
				PreviousHash: first.PreviousBlock,
				MerkleRoot:   first.MerkleRoot,
				Time:         first.Timestamp,
				Bits:         first.DifficultyTarget,
				Nonce:        first.Nonce,
			},
			Hash: first.Hash,
		}

		// when:
		actualHeader, err := svc.ChainHeaderByHeight(t.Context(), testservices.TestBlockHeight)

		// then:
		require.NoError(t, err)
		require.Equal(t, expectedHeader, actualHeader)
	})
}

func TestGetChainHeaderByHeight_NegativePaths(t *testing.T) {
	t.Run("return error when all services are unreachable", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		_ = given.Bitails().WillBeUnreachable()
		_ = given.WhatsOnChain().WillBeUnreachable()
		expectedSubstr := given.BHS().WillBeUnreachable().Error()
		_ = given.Chaintracks().WillFail()

		// and:
		services := given.Services().Config(
			testservices.WithEnabledBitails(true),
			testservices.WithEnabledBHS(true),
			testservices.WithEnabledChaintracks(true),
		).New()

		// when:
		header, err := services.ChainHeaderByHeight(t.Context(), testservices.TestBlockHeight)

		// then:
		testabilities.IsNotMockTransportResponderError(t, err)

		require.ErrorContains(t, err, expectedSubstr)
		assert.Nil(t, header)
	})

	t.Run("return an error when all block header services respond with internal server error", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.BHS().WillRespondWithInternalFailure()
		given.WhatsOnChain().WillRespondWithInternalFailure()
		given.Bitails().WillRespondWithInternalFailure()
		_ = given.Chaintracks().WillFail()

		// and:
		services := given.Services().Config(
			testservices.WithEnabledBitails(true),
			testservices.WithEnabledBHS(true),
			testservices.WithEnabledChaintracks(true),
		).New()

		// when:
		response, err := services.ChainHeaderByHeight(t.Context(), testservices.TestBlockHeight)

		// then:
		testabilities.IsNotMockTransportResponderError(t, err)

		require.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("return an error when all block header services return an empty header blocks response", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.BHS().WillRespondWithEmptyBlockHeight()
		given.WhatsOnChain().WillRespondWithEmptyBlockHeight()
		given.Bitails().WillRespondWithEmptyBlockHeight()
		_ = given.Chaintracks().WillFail()

		// and:
		service := given.Services().Config(
			testservices.WithEnabledBitails(true),
			testservices.WithEnabledBHS(true),
			testservices.WithEnabledChaintracks(true),
		).New()

		// when:
		actualBlock, err := service.ChainHeaderByHeight(t.Context(), 0) // Assuming height 0 for empty response scenario

		// then:
		testabilities.IsNotMockTransportResponderError(t, err)

		require.Error(t, err)
		require.Nil(t, actualBlock)
	})
}
