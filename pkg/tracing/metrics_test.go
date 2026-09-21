package tracing_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
)

func TestEnableMetrics_BuildsProvider(t *testing.T) {
	// when: the OTLP exporter dials lazily, so no collector is needed
	cleanup, err := tracing.EnableMetrics(logging.NewTestLogger(t), "test", "http://localhost:4317", time.Minute)

	// then: resource construction must not fail (e.g. on conflicting schema URLs)
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	// cleanup is not called: shutdown would try to flush to the absent collector.
}
