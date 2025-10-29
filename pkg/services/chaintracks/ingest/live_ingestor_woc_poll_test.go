package ingest_test

import (
	_ "embed"
	"fmt"
	"testing"
	"time"

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
	blockHash, responseBodyMap := blockHeaderStandardResponse(t)
	config := defs.DefaultWOCPollIngestorConfig()

	mockWOC := testabilities.GivenMockWOC(t, config.Chain)
	mockWOC.WillRespondOn(fmt.Sprintf("block/%s/header", blockHash), "GET").WithJSONResponse(200, responseBodyMap)

	ingestor := ingest.NewLiveIngestorWocPoll(logging.NewTestLogger(t), config, ingest.WithRestyClient(mockWOC.HttpClient()))

	// when:
	resp, err := ingestor.GetHeaderByHash(t.Context(), blockHash)

	// then:
	require.NoError(t, err)
	assert.Equal(t, blockHash, resp.Hash)
	assert.Equal(t, uint(920784), resp.Height)
	assert.Equal(t, uint32(603979776), resp.Version)
	assert.Equal(t, "5d8647e91f98e7518bb7a2e4208e477a499692893fa6f74b0748e6e3bf6e5083", resp.MerkleRoot)
	assert.Equal(t, uint32(1761730369), resp.Time)
	assert.Equal(t, uint32(3748417179), resp.Nonce)
	assert.Equal(t, uint32(0x1822d1cf), resp.Bits)
	assert.Equal(t, "000000000000000008f00a66296132fb60e173ad29dd4c50465d51b0e5530366", resp.PreviousHash)
}

func TestLiveIngestorWOCPoll_BlockHeaderPrevHashEmpty(t *testing.T) {
	// given:
	blockHash, responseBodyMap := blockHeaderStandardResponse(t)
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
	blockHash, _ := blockHeaderStandardResponse(t)
	config := defs.DefaultWOCPollIngestorConfig()

	mockWOC := testabilities.GivenMockWOC(t, config.Chain)
	mockWOC.WillRespondOn(fmt.Sprintf("block/%s/header", blockHash), "GET").WithJSONResponse(404, map[string]any{})

	ingestor := ingest.NewLiveIngestorWocPoll(logging.NewTestLogger(t), config, ingest.WithRestyClient(mockWOC.HttpClient()))

	// when:
	_, err := ingestor.GetHeaderByHash(t.Context(), blockHash)

	// then:
	require.ErrorIs(t, err, wdk.ErrNotFoundError)
}

func TestLiveIngestorWOCPoll_PollLast10Headers(t *testing.T) {
	// given:
	last10Headers := testabilities.WOCLast10Headers(t)
	config := defs.DefaultWOCPollIngestorConfig()

	mockWOC := testabilities.GivenMockWOC(t, config.Chain)
	mockWOC.WillRespondOn("block/headers", "GET").WithJSONResponse(200, last10Headers)

	ingestor := ingest.NewLiveIngestorWocPoll(logging.NewTestLogger(t), config, ingest.WithRestyClient(mockWOC.HttpClient()))

	// when:
	mockReceiver := testabilities.NewMockHeadersReceiver(t)
	ingestor.StartListening(t.Context(), mockReceiver.RespChan)

	// then:
	require.Eventually(t, func() bool {
		return len(mockReceiver.GetReceivedHeaders()) == 10
	}, 2*time.Second, 100*time.Millisecond)

	// when:
	ingestor.StopListening()
}

func TestLiveIngestorWOCPoll_PollLast10Headers_Twice(t *testing.T) {
	// given:
	last10Headers := testabilities.WOCLast10Headers(t)
	config := defs.DefaultWOCPollIngestorConfig()

	mockWOC := testabilities.GivenMockWOC(t, config.Chain)
	mockWOC.WillRespondOn("block/headers", "GET").WithJSONResponse(200, last10Headers)

	ingestor := ingest.NewLiveIngestorWocPoll(logging.NewTestLogger(t), config, ingest.WithRestyClient(mockWOC.HttpClient()), ingest.WithSyncPeriod(100*time.Millisecond))

	// when:
	mockReceiver := testabilities.NewMockHeadersReceiver(t)
	ingestor.StartListening(t.Context(), mockReceiver.RespChan)

	// then:
	require.Eventually(t, func() bool {
		return len(mockReceiver.GetReceivedHeaders()) == 20
	}, 2*time.Second, 100*time.Millisecond)

	// when:
	ingestor.StopListening()
}

func TestLiveIngestorWOCPoll_PollLast10Headers_TemporaryFailsDontBother(t *testing.T) {
	// given:
	last10Headers := testabilities.WOCLast10Headers(t)
	config := defs.DefaultWOCPollIngestorConfig()

	mockWOC := testabilities.GivenMockWOC(t, config.Chain)

	// and first make it fail
	mockWOC.WillRespondOn("block/headers", "GET").WithJSONResponse(404, map[string]any{})

	ingestor := ingest.NewLiveIngestorWocPoll(logging.NewTestLogger(t), config, ingest.WithRestyClient(mockWOC.HttpClient()), ingest.WithSyncPeriod(100*time.Millisecond))

	// when:
	mockReceiver := testabilities.NewMockHeadersReceiver(t)
	ingestor.StartListening(t.Context(), mockReceiver.RespChan)

	time.Sleep(200 * time.Millisecond)
	// now make it work
	mockWOC.WillRespondOn("block/headers", "GET").WithJSONResponse(200, last10Headers)

	// then:
	require.Eventually(t, func() bool {
		return len(mockReceiver.GetReceivedHeaders()) >= 10
	}, 5*time.Second, 100*time.Millisecond)

	// when:
	ingestor.StopListening()
}

func blockHeaderStandardResponse(t *testing.T) (string, map[string]any) {
	t.Helper()
	blockHeader := testabilities.WOCLastHeader(t)
	blockHash, ok := blockHeader["hash"].(string)
	require.True(t, ok)
	return blockHash, blockHeader
}
