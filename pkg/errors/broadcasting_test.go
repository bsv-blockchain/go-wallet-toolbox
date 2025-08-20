package errors_test

import (
	"errors"
	"fmt"
	"testing"

	broadcastError "github.com/bsv-blockchain/go-wallet-toolbox/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBroadcastingError_Is(t *testing.T) {
	t.Run("should support errors.Is with underlying error", func(t *testing.T) {
		originalErr := fmt.Errorf("connection timeout")
		wrappedErr := fmt.Errorf("service failed: %w", originalErr)

		broadcastErr := broadcastError.NewBroadcastingError(wrappedErr, "createAction")

		assert.True(t, errors.Is(broadcastErr, originalErr))
		assert.True(t, errors.Is(broadcastErr, wrappedErr))

		anotherBroadcastErr := &broadcastError.BroadcastingError{}
		assert.True(t, errors.Is(broadcastErr, anotherBroadcastErr))
	})

	t.Run("should support errors.As for type assertion", func(t *testing.T) {
		originalErr := fmt.Errorf("service unavailable")
		broadcastErr := broadcastError.NewBroadcastingError(originalErr, broadcastError.ProcessAction)
		broadcastErr.TxID = "test-tx-id"
		broadcastErr.Reference = "test-reference"

		wrappedErr := fmt.Errorf("operation failed: %w", broadcastErr)

		var extractedErr *broadcastError.BroadcastingError
		require.True(t, errors.As(wrappedErr, &extractedErr))

		assert.Equal(t, "test-tx-id", extractedErr.TxID)
		assert.Equal(t, "test-reference", extractedErr.Reference)
		assert.Equal(t, broadcastError.ProcessAction, extractedErr.Operation)
	})
}
