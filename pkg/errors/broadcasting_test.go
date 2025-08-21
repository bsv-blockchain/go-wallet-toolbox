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
		// given:
		originalErr := fmt.Errorf("connection timeout")
		wrappedErr := fmt.Errorf("service failed: %w", originalErr)
		broadcastErr := broadcastError.NewBroadcastingError(wrappedErr, broadcastError.CreateAction)
		anotherBroadcastErr := &broadcastError.BroadcastingError{}

		// when:
		isOriginal := errors.Is(broadcastErr, originalErr)
		isWrapped := errors.Is(broadcastErr, wrappedErr)
		isType := errors.Is(broadcastErr, anotherBroadcastErr)

		// then:
		assert.True(t, isOriginal)
		assert.True(t, isWrapped)
		assert.True(t, isType)
	})

	t.Run("should support errors.As for type assertion", func(t *testing.T) {
		// given:
		originalErr := fmt.Errorf("service unavailable")
		broadcastErr := broadcastError.NewBroadcastingError(originalErr, broadcastError.ProcessAction)
		broadcastErr.TxID = "test-tx-id"
		broadcastErr.Reference = "test-reference"
		wrappedErr := fmt.Errorf("operation failed: %w", broadcastErr)

		// when:
		var extractedErr *broadcastError.BroadcastingError
		require.True(t, errors.As(wrappedErr, &extractedErr))

		// then:
		assert.Equal(t, "test-tx-id", extractedErr.TxID)
		assert.Equal(t, "test-reference", extractedErr.Reference)
		assert.Equal(t, broadcastError.ProcessAction, extractedErr.Operation)
	})
}
