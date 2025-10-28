package ingest_test

import (
	"fmt"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiveIngestorWOCPoll_BlockHeaderSuccessfulResp(t *testing.T) {
	// given:
	blockHash, responseBodyMap := blockHeaderStandardResponse()
	config := defs.DefaultWOCPollIngestorConfig()

	mockWOC := testabilities.GivenMockWOC(t, config.Chain)
	mockWOC.WillRespondOn(fmt.Sprintf("block/%s/header", blockHash), "GET").WithJSONResponse(200, responseBodyMap)

	ingestor := ingest.NewLiveIngestorWocPoll(logging.NewTestLogger(t), config, ingest.WithRestyClient(mockWOC.HttpClient()))

	// when:
	resp, err := ingestor.GetHeaderByHash(t.Context(), blockHash)

	// then:
	require.NoError(t, err)
	assert.Equal(t, blockHash, resp.Hash)
	assert.Equal(t, uint(920621), resp.Height)
	assert.Equal(t, uint32(576192512), resp.Version)
	assert.Equal(t, "b390a2971e86e9c44defac54b686017b37e6d9e2f6a1e95f59d9564dd1de69eb", resp.MerkleRoot)
	assert.Equal(t, uint32(1761641684), resp.Time)
	assert.Equal(t, uint32(1869848676), resp.Nonce)
	assert.Equal(t, uint32(0x1829e687), resp.Bits)
	assert.Equal(t, "000000000000000025e9f8bb962e3c9426f7d18754404d848f6a1ed867ca10a8", resp.PreviousHash)
}

func TestLiveIngestorWOCPoll_BlockHeaderPrevHashEmpty(t *testing.T) {
	// given:
	blockHash, responseBodyMap := blockHeaderStandardResponse()
	responseBodyMap["previousblockhash"] = ""

	config := defs.DefaultWOCPollIngestorConfig()

	mockWOC := testabilities.GivenMockWOC(t, config.Chain)
	mockWOC.WillRespondOn(fmt.Sprintf("block/%s/header", blockHash), "GET").WithJSONResponse(200, responseBodyMap)

	ingestor := ingest.NewLiveIngestorWocPoll(logging.NewTestLogger(t), config, ingest.WithRestyClient(mockWOC.HttpClient()))

	// when:
	resp, err := ingestor.GetHeaderByHash(t.Context(), blockHash)

	// then:
	require.NoError(t, err)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000000000000", resp.PreviousHash)
}

func TestLiveIngestorWOCPoll_BlockHeaderNotFound(t *testing.T) {
	// given:
	blockHash, _ := blockHeaderStandardResponse()
	config := defs.DefaultWOCPollIngestorConfig()

	mockWOC := testabilities.GivenMockWOC(t, config.Chain)
	mockWOC.WillRespondOn(fmt.Sprintf("block/%s/header", blockHash), "GET").WithJSONResponse(404, map[string]any{})

	ingestor := ingest.NewLiveIngestorWocPoll(logging.NewTestLogger(t), config, ingest.WithRestyClient(mockWOC.HttpClient()))

	// when:
	_, err := ingestor.GetHeaderByHash(t.Context(), blockHash)

	// then:
	require.ErrorIs(t, err, wdk.ErrNotFoundError)
}

func blockHeaderStandardResponse() (string, map[string]any) {
	blockHash := "0000000000000000036543a8346f24fa5fb4afd66999be8fa891ed351b7149df"
	return blockHash, map[string]any{
		"hash":              blockHash,
		"confirmations":     int64(10),
		"size":              int64(145580460),
		"height":            int64(920621),
		"version":           int64(576192512),
		"versionHex":        "22580000",
		"merkleroot":        "b390a2971e86e9c44defac54b686017b37e6d9e2f6a1e95f59d9564dd1de69eb",
		"time":              int64(1761641684),
		"mediantime":        int64(1761639320),
		"nonce":             int64(1869848676),
		"bits":              "1829e687",
		"difficulty":        26240615692.58609, // float64
		"chainwork":         "0000000000000000000000000000000000000000016961148da806521ced57bd",
		"previousblockhash": "000000000000000025e9f8bb962e3c9426f7d18754404d848f6a1ed867ca10a8",
		"nextblockhash":     "0000000000000000035c7f80189bea5747b88313cd88bbe5a73c265cabaefe47",
	}
}
