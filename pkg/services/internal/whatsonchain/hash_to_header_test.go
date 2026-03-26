package whatsonchain_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
)

func TestWhatsOnChain_HashToHeader_Success(t *testing.T) {
	// given:
	given := testabilities.Given(t)

	blockHash := testabilities.TestTargetHash
	blockHeight := testabilities.TestBlockHeight
	version := 536870912
	merkleRoot := testabilities.TestMerkleRootHex
	time := uint32(1712345678)
	nonce := 123456789
	bits := "1803a30c"
	prevHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	mockResponse := testabilities.ValidBlockHeaderJSON(
		blockHash, blockHeight, version, merkleRoot, time, nonce, bits, prevHash)

	given.WhatsOnChain().WillRespondWithBlockHeader(http.StatusOK, blockHash, mockResponse)

	svc := given.NewWoCService()

	// when:
	res, err := svc.HashToHeader(t.Context(), blockHash)

	// then:
	require.NoError(t, err)
	assert.Equal(t, blockHash, res.Hash)
	assert.Equal(t, uint(blockHeight), res.Height)
	assert.Equal(t, version, int(res.Version))
	assert.Equal(t, merkleRoot, res.MerkleRoot)
	assert.Equal(t, time, res.Time)
	assert.Equal(t, nonce, int(res.Nonce))
	assert.Equal(t, prevHash, res.PreviousHash)
}

func TestWhatsOnChain_HashToHeader_ErrorStatus(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	blockHash := testabilities.TestTargetHash

	given.WhatsOnChain().WillRespondWithBlockHeader(http.StatusInternalServerError, blockHash, "unexpected")

	svc := given.NewWoCService()

	// when:
	_, err := svc.HashToHeader(t.Context(), blockHash)

	// then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response status")
}

func TestWhatsOnChain_HashToHeader_InvalidBits(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	blockHash := testabilities.TestTargetHash

	mockResponse := testabilities.InvalidBitsBlockHeaderJSON(blockHash, testabilities.TestMerkleRootHex)

	given.WhatsOnChain().WillRespondWithBlockHeader(http.StatusOK, blockHash, mockResponse)

	svc := given.NewWoCService()

	// when:
	_, err := svc.HashToHeader(t.Context(), blockHash)

	// then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid bits value")
}

func TestWhatsOnChain_HashToHeader_HTTPError(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	blockHash := testabilities.TestTargetHash

	given.WhatsOnChain().WillRespondWithBlockHeader(http.StatusInternalServerError, blockHash, "Internal Server Error")

	svc := given.NewWoCService()

	// when:
	_, err := svc.HashToHeader(t.Context(), blockHash)

	// then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response status")
}

func TestWhatsOnChain_HashToHeader_InvalidJSON(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	blockHash := testabilities.TestTargetHash

	given.WhatsOnChain().WillRespondWithBlockHeader(http.StatusOK, blockHash, `invalid-json}`)

	svc := given.NewWoCService()

	// when:
	_, err := svc.HashToHeader(t.Context(), blockHash)

	// then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch block header")
}

func TestWhatsOnChain_HashToHeader_MissingFields(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	blockHash := testabilities.TestTargetHash

	mockResponse := testabilities.IncompleteBlockHeaderJSON(testabilities.TestTargetHash, testabilities.TestMerkleRootHex)

	given.WhatsOnChain().WillRespondWithBlockHeader(http.StatusOK, blockHash, mockResponse)

	svc := given.NewWoCService()

	// when:
	_, err := svc.HashToHeader(t.Context(), blockHash)

	// then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid bits value")
}
