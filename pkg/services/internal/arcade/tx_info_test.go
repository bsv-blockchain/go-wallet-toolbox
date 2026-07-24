package arcade_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arcade"
)

func TestAllStatusesMatchesArcadeLifecycle(t *testing.T) {
	// Mirrors github.com/bsv-blockchain/arcade models.AllStatuses order.
	want := []arcade.TxStatus{
		arcade.StatusUnknown,
		arcade.StatusReceived,
		arcade.StatusSentToNetwork,
		arcade.StatusAcceptedByNetwork,
		arcade.StatusSeenOnNetwork,
		arcade.StatusSeenMultipleNodes,
		arcade.StatusDoubleSpendAttempted,
		arcade.StatusRejected,
		arcade.StatusPendingRetry,
		arcade.StatusStumpProcessing,
		arcade.StatusMined,
		arcade.StatusImmutable,
	}
	got := arcade.AllStatuses()
	require.Equal(t, want, got)

	// Canonical multi-node name (live SSE); legacy alias still defined.
	assert.Equal(t, arcade.TxStatus("SEEN_MULTIPLE_NODES"), arcade.StatusSeenMultipleNodes)
	assert.Equal(t, arcade.TxStatus("SEEN_ON_MULTIPLE_NODES"), arcade.StatusSeenOnMultipleNodes)
	assert.Equal(t, arcade.TxStatus("ACCEPTED_BY_NETWORK"), arcade.StatusAcceptedByNetwork)
}
