package chaintracks_test

import (
	"log/slog"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Lifecycle(t *testing.T) {
	// IMPORTANT NOTE:
	// This test does ACTUAL calls to the 3rd party services. For now, it's intended.
	// This way we can prove that the initial syncing is working in real life conditions.
	// In the future, when things are more stable, we might want to mock the 3rd party services here as well.
	// given:
	service, err := chaintracks.NewService(logging.NewTestLogger(t), defs.DefaultChaintracksServiceConfig())
	require.NoError(t, err)

	// when:
	err = service.MakeAvailable(t.Context())

	// then:
	require.NoError(t, err, "make available should not return error")
	require.True(t, service.Available(), "service should be available after make available")

	// when:
	height, err := service.GetPresentHeight(t.Context())

	// then:
	require.NoError(t, err, "get present height should not return error")
	require.Greater(t, height, uint(900000), "present height should be greater than 900000")

	t.Run("GetInfo", func(t *testing.T) {
		// when:
		info, err := service.GetInfo(t.Context())

		// then:
		require.NoError(t, err, "get info should not return error")
		assert.Equal(t, defs.NetworkMainnet, info.Chain)
		assert.Equal(t, "gorm-sqlite-inmemory", info.Storage)
		assert.Equal(t, []string{"woc_poll"}, info.LiveIngestors)
		assert.Equal(t, []string{"chaintracks_cdn (source_url=https://cdn.projectbabbage.com/blockheaders)", "whats_on_chain_cdn"}, info.BulkIngestors)
		t.Logf("Bulk height: %d", info.HeightBulk) // TODO: When we use live data (not mocked), this value is not constant; will be changed to assert.Equal later
		t.Logf("Live height: %d", info.HeightLive)
	})

	t.Run("FindChainTipHeader", func(t *testing.T) {
		// when:
		tipHeader, err := service.FindChainTipHeader(t.Context())

		// then:
		require.NoError(t, err, "find chain tip header should not return error")
		require.NotNil(t, tipHeader, "tip header should not be nil")
		require.Equal(t, height, tipHeader.Height, "tip header height should be equal to present height")
	})

	t.Run("FindHeaderForHeight_recent", func(t *testing.T) {
		// when:
		header, err := service.FindHeaderForHeight(t.Context(), height-10)

		// then:
		require.NoError(t, err, "find header for height should not return error")
		require.NotNil(t, header, "header should not be nil")
		require.Equal(t, height-10, header.Height, "header height should be equal to requested height")
		t.Logf("Header at height %d: %#+v", header.Height, header)
	})

	t.Run("FindHeaderForHeight_genesis", func(t *testing.T) {
		// when:
		header, err := service.FindHeaderForHeight(t.Context(), 0)

		// then:
		require.NoError(t, err, "find header for height 0 should not return error")
		require.NotNil(t, header, "header at height 0 should not be nil")
		require.Equal(t, uint(0), header.Height, "header height should be 0")
		require.Equal(t, "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f", header.Hash, "header hash at height 0 should be genesis hash")
		t.Logf("Genesis header: %#+v", header)
	})

	t.Run("FindHeaderForHeight_old", func(t *testing.T) {
		// when:
		header, err := service.FindHeaderForHeight(t.Context(), height-5000)

		// then:
		require.NoError(t, err, "find header for height should not return error")
		require.NotNil(t, header, "header should not be nil")
		require.Equal(t, height-5000, header.Height, "header height should be equal to requested height")
		t.Logf("Header at height %d: %#+v", header.Height, header)
	})

	// when:
	service.Destroy()

	// then:
	require.False(t, service.Available(), "service should be unavailable after destroy")
}

func TestService_GetPresentHeight(t *testing.T) {
	t.Skip("Skipping TestService_GetPresentHeight till we implement whole mocking or it (WoC live/bulk + Babbage bulk)")
	// given:
	const expectedHeight = 920784
	config := defs.DefaultChaintracksServiceConfig()

	mockWOC := testabilities.GivenMockWOC(t, config.Chain)
	mockWOC.WillRespondOn("chain/info", "GET").WithJSONResponse(200, map[string]any{"blocks": expectedHeight})

	// and:
	service, err := chaintracks.NewService(logging.NewTestLogger(t), config, chaintracks.Initializers{
		WOCLiveIngestorPollFactory: func(logger *slog.Logger, config defs.ChaintracksServiceConfig) chaintracks.LiveIngestor {
			return ingest.NewLiveIngestorWocPoll(logger, config.Chain, ingest.IngestorWocPollOpts.WithRestyClient(mockWOC.HttpClient()))
		},
	})
	require.NoError(t, err)

	//and:
	err = service.MakeAvailable(t.Context())
	require.NoError(t, err)

	// when:
	presentHeight, err := service.GetPresentHeight(t.Context())

	// then:
	require.NoError(t, err)
	require.Equal(t, uint(expectedHeight), presentHeight)

	// and:
	require.Equal(t, 1, mockWOC.ServicesSniffer().CountCallsByRegex(`/chain/info`))

	// when:
	presentHeight, err = service.GetPresentHeight(t.Context())

	// then:
	require.NoError(t, err)
	require.Equal(t, uint(expectedHeight), presentHeight)

	// and, the call count should not have increased since the result should be cached
	require.Equal(t, 1, mockWOC.ServicesSniffer().CountCallsByRegex(`/chain/info`))

	// clean up:
	service.Destroy()
}
