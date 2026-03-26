package bitails_test

import (
	"net/http"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
)

func TestBitails_HashToHeader_Success(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	blockHash := testabilities.TestHex
	rawHeader := testabilities.ValidBlockHeaderRaw()

	given.Bitails().WillReturnBlockHeader(blockHash, rawHeader)
	svc := given.NewBitailsService()

	// when:
	res, err := svc.HashToHeader(t.Context(), blockHash)

	// then:
	require.NoError(t, err)
	assert.Equal(t, blockHash, res.Hash)
	assert.Equal(t, uint(testabilities.TestZeroHeaderHeight), res.Height)
	assert.Equal(t, testabilities.TestHeaderHex, res.MerkleRoot)
}

func TestBitails_HashToHeader_HTTPError(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	blockHash := testabilities.TestTargetHash

	given.Bitails().WillReturnBlockHeaderHttpError(blockHash, http.StatusInternalServerError)
	svc := given.NewBitailsService()

	// when:
	_, err := svc.HashToHeader(t.Context(), blockHash)

	// then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch raw header")
}

func TestBitails_HashToHeader_InvalidHex(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	blockHash := testabilities.TestTargetHash
	rawHeader := "INVALIDHEX"

	given.Bitails().WillReturnBlockHeader(blockHash, rawHeader)
	svc := given.NewBitailsService()

	// when:
	_, err := svc.HashToHeader(t.Context(), blockHash)

	// then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error decoding block header hex")
}

func TestBitails_HashToHeader_RequestError(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	blockHash := testabilities.TestTargetHash

	err := given.Bitails().WillBeUnreachable()
	require.Error(t, err, "failed to set up Bitails service to be unreachable")

	svc := given.NewBitailsService()

	// when:
	_, err = svc.HashToHeader(t.Context(), blockHash)

	// then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch raw header")
}

func TestBitails_HashToHeader_InvalidJSON(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	blockHash := testabilities.TestTargetHash

	given.Bitails().WillReturnMalformedBlockHeader(blockHash)
	svc := given.NewBitailsService()

	// when:
	_, err := svc.HashToHeader(t.Context(), blockHash)

	// then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch raw header")
}

func TestBitails_HashToHeader_MissingFields(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	blockHash := testabilities.TestTargetHash
	rawHeader := testabilities.IncompleteBlockHeaderRaw()

	given.Bitails().WillReturnBlockHeader(blockHash, rawHeader)
	svc := given.NewBitailsService()

	// when:
	_, err := svc.HashToHeader(t.Context(), blockHash)

	// then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected 80-byte block header")
}

func TestBitails_HashToHeader(t *testing.T) {
	t.Run("successfully converts raw header", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		svc := given.NewBitailsService()

		rawHeaderHex := testabilities.TestFakeHeaderBinary
		rawHeader := testabilities.MustDecodeHex(t, rawHeaderHex)
		expectedHash := chainhash.DoubleHashH(rawHeader)

		given.Bitails().WillReturnBlockHeader(expectedHash.String(), rawHeaderHex)

		// when:
		result, err := svc.HashToHeader(t.Context(), expectedHash.String())

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, expectedHash.String(), result.Hash)
		assert.Equal(t, uint(0), result.Height)
	})

	t.Run("returns error on HTTP failure", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		svc := given.NewBitailsService()

		blockHash := testabilities.TestTargetHash
		given.Bitails().WillReturnBlockHeaderHttpError(blockHash, http.StatusInternalServerError)

		// when:
		result, err := svc.HashToHeader(t.Context(), blockHash)

		// then:
		require.Error(t, err)
		require.Nil(t, result)
	})

	t.Run("returns error on invalid hex", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		svc := given.NewBitailsService()

		blockHash := testabilities.TestTargetHash
		given.Bitails().WillReturnBlockHeader(blockHash, "zzzz_not_valid_hex")

		// when:
		result, err := svc.HashToHeader(t.Context(), blockHash)

		// then:
		require.Error(t, err)
		require.Nil(t, result)
	})

	t.Run("returns error on incorrect length", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		svc := given.NewBitailsService()

		blockHash := testabilities.TestTargetHash
		given.Bitails().WillReturnBlockHeader(blockHash, "00")

		// when:
		result, err := svc.HashToHeader(t.Context(), blockHash)

		// then:
		require.Error(t, err)
		require.Nil(t, result)
	})
}
