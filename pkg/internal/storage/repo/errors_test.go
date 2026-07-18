package repo_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// TestErrUTXOContention_WrapsPublicSentinel asserts that the storage-layer
// repo.ErrUTXOContention wraps the public wdk.ErrUTXOContention sentinel, so
// callers outside this package (e.g. actions.create.Create's retry loop) can
// match on it via errors.Is(err, wdk.ErrUTXOContention) without needing to
// import the internal repo package.
func TestErrUTXOContention_WrapsPublicSentinel(t *testing.T) {
	require.ErrorIs(t, repo.ErrUTXOContention, wdk.ErrUTXOContention)

	// and: an additional layer of %w-wrapping, as reserveUTXOs and
	// markReservedOutputsAsNotSpendable actually do, still resolves down to the
	// public sentinel.
	wrapped := fmt.Errorf("%w: expected %d, got %d", repo.ErrUTXOContention, 2, 1)
	require.ErrorIs(t, wrapped, wdk.ErrUTXOContention)
	require.ErrorIs(t, wrapped, repo.ErrUTXOContention)
}
