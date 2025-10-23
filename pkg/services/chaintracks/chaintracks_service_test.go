package chaintracks_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks"
	"github.com/stretchr/testify/require"
)

func TestChaintracksService_Lifecycle(t *testing.T) {
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
