package chaintracks_test

import (
	"log/slog"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest/testabilities"
	"github.com/stretchr/testify/require"
)

func TestService_Lifecycle(t *testing.T) {
	// given:
	service, err := chaintracks.NewService(logging.NewTestLogger(t), defs.DefaultChaintracksServiceConfig())
	require.NoError(t, err)

	// when:
	err = service.MakeAvailable(t.Context())

	// then:
	require.NoError(t, err, "make available should not return error")
	require.True(t, service.Available(), "service should be available after make available")

	// when:
	service.Destroy()

	// then:
	require.False(t, service.Available(), "service should be unavailable after destroy")
}

func TestService_GetPresentHeight(t *testing.T) {
	// given:
	const expectedHeight = 920784
	config := defs.DefaultChaintracksServiceConfig()

	mockWOC := testabilities.GivenMockWOC(t, config.Chain)
	mockWOC.WillRespondOn("chain/info", "GET").WithJSONResponse(200, map[string]any{"blocks": expectedHeight})

	// and:
	service, err := chaintracks.NewService(logging.NewTestLogger(t), config, chaintracks.Initializers{
		WOCLiveIngestorPollFactory: func(logger *slog.Logger, config defs.ChaintracksServiceConfig) chaintracks.LiveIngestor {
			return ingest.NewLiveIngestorWocPoll(logger, defs.WOCPollIngestorConfig{Chain: config.Chain}, ingest.WithRestyClient(mockWOC.HttpClient()))
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
	require.Equal(t, uint32(expectedHeight), presentHeight)

	// and:
	require.Equal(t, 1, mockWOC.ServicesSniffer().CountCallsByRegex(`/chain/info`))

	// when:
	presentHeight, err = service.GetPresentHeight(t.Context())

	// then:
	require.NoError(t, err)
	require.Equal(t, uint32(expectedHeight), presentHeight)

	// and, the call count should not have increased since the result should be cached
	require.Equal(t, 1, mockWOC.ServicesSniffer().CountCallsByRegex(`/chain/info`))

	// clean up:
	service.Destroy()
}

func TestService_GetPresentHeight_FirstFailed_SecondSucceded(t *testing.T) {
	// given:
	const expectedHeight = 920784
	config := defs.DefaultChaintracksServiceConfig()

	mockWOC := testabilities.GivenMockWOC(t, config.Chain)
	mockWOC.WillRespondOn("chain/info", "GET").WithJSONResponse(500, map[string]any{"error": 500})

	// and:
	service, err := chaintracks.NewService(logging.NewTestLogger(t), config, chaintracks.Initializers{
		WOCLiveIngestorPollFactory: func(logger *slog.Logger, config defs.ChaintracksServiceConfig) chaintracks.LiveIngestor {
			return ingest.NewLiveIngestorWocPoll(logger, defs.WOCPollIngestorConfig{Chain: config.Chain}, ingest.WithRestyClient(mockWOC.HttpClient()))
		},
	})
	require.NoError(t, err)

	//and:
	err = service.MakeAvailable(t.Context())
	require.NoError(t, err)

	// when:
	_, err = service.GetPresentHeight(t.Context())

	// then:
	require.Error(t, err)

	// and:
	require.Equal(t, 1, mockWOC.ServicesSniffer().CountCallsByRegex(`/chain/info`))

	// when:
	mockWOC.WillRespondOn("chain/info", "GET").WithJSONResponse(200, map[string]any{"blocks": expectedHeight})
	presentHeight, err := service.GetPresentHeight(t.Context())

	// then:
	require.NoError(t, err)
	require.Equal(t, uint32(expectedHeight), presentHeight)

	// and, the call count should have increased since the first call failed
	require.Equal(t, 2, mockWOC.ServicesSniffer().CountCallsByRegex(`/chain/info`))

	// clean up:
	service.Destroy()
}
