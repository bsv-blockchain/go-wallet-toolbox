package errors_test

import (
	"errors"
	"fmt"
	"testing"

	broadcastError "github.com/bsv-blockchain/go-wallet-toolbox/pkg/errors"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBroadcastingError(t *testing.T) {
	t.Run("should format complete error message with all context", func(t *testing.T) {
		// given:
		broadcastErr := &broadcastError.BroadcastingError{
			Err:       fmt.Errorf("underlying network error"),
			TxID:      "abc123def456",
			Reference: "ref-789",
			Operation: broadcastError.ImmediateBroadcast,
			ServiceErrors: map[string]error{
				"service1": fmt.Errorf("service1 timeout"),
				"service2": fmt.Errorf("service2 unavailable"),
			},
			SendWithResults: []wdk.SendWithResult{
				{TxID: "abc123", Status: wdk.SendWithResultStatusUnproven},
				{TxID: "def456", Status: wdk.SendWithResultStatusFailed},
				{TxID: "ghi789", Status: wdk.SendWithResultStatusUnproven},
			},
		}

		// when:
		errorMessage := broadcastErr.Error()

		// then:
		assert.Contains(t, errorMessage, "broadcasting failed during immediateBroadcast")
		assert.Contains(t, errorMessage, "for txID abc123def456")
		assert.Contains(t, errorMessage, "(reference: ref-789)")
		assert.Contains(t, errorMessage, "transactions: 3 total, 2 succeeded, 1 failed")
		assert.Contains(t, errorMessage, "service errors: 2 services failed")
		assert.Contains(t, errorMessage, "underlying error: underlying network error")
	})

	t.Run("should format minimal error message with only required fields", func(t *testing.T) {
		// given:
		broadcastErr := &broadcastError.BroadcastingError{
			Err:       fmt.Errorf("simple error"),
			Operation: broadcastError.CreateAction,
		}

		// when:
		errorMessage := broadcastErr.Error()

		// then:
		assert.Contains(t, errorMessage, "broadcasting failed during createAction")
		assert.Contains(t, errorMessage, "underlying error: simple error")
		assert.NotContains(t, errorMessage, "txID")
		assert.NotContains(t, errorMessage, "reference")
		assert.NotContains(t, errorMessage, "transactions:")
		assert.NotContains(t, errorMessage, "service errors:")
	})

	t.Run("should format error message without underlying error", func(t *testing.T) {
		// given:
		broadcastErr := &broadcastError.BroadcastingError{
			TxID:      "test-tx",
			Operation: broadcastError.ProcessAction,
		}

		// when:
		errorMessage := broadcastErr.Error()

		// then:
		assert.Contains(t, errorMessage, "broadcasting failed during processAction")
		assert.Contains(t, errorMessage, "for txID test-tx")
		assert.NotContains(t, errorMessage, "underlying error:")
	})
}

func TestBroadcastingErrorUnwrap(t *testing.T) {
	t.Run("should return the underlying error", func(t *testing.T) {
		// given:
		underlyingErr := fmt.Errorf("network timeout")
		broadcastErr := &broadcastError.BroadcastingError{
			Err: underlyingErr,
		}

		// when:
		unwrappedErr := errors.Unwrap(broadcastErr)

		// then:
		assert.Equal(t, underlyingErr, unwrappedErr)
	})

	t.Run("should return nil when no underlying error", func(t *testing.T) {
		// given:
		broadcastErr := &broadcastError.BroadcastingError{}

		// when:
		unwrappedErr := errors.Unwrap(broadcastErr)

		// then:
		assert.Nil(t, unwrappedErr)
	})
}

func TestBroadcastingErrorIs(t *testing.T) {
	t.Run("should support errors.Is with underlying error chain", func(t *testing.T) {
		// given:
		originalErr := fmt.Errorf("connection timeout")
		wrappedErr := fmt.Errorf("service failed: %w", originalErr)
		broadcastErr := &broadcastError.BroadcastingError{
			Err: wrappedErr,
		}

		// when:
		matchOrginalErr := errors.Is(broadcastErr, originalErr)
		matchWrappedErr := errors.Is(broadcastErr, wrappedErr)
		matchUnrelatedErr := errors.Is(broadcastErr, fmt.Errorf("unrelated error"))

		// then:
		assert.True(t, matchOrginalErr, "should match original error")
		assert.True(t, matchWrappedErr, "should match wrapped error")
		assert.False(t, matchUnrelatedErr, "should not match unrelated error")
	})

	t.Run("should support errors.Is with BroadcastingError type", func(t *testing.T) {
		// given:
		broadcastErr := &broadcastError.BroadcastingError{
			Err: fmt.Errorf("some error"),
		}
		anotherBroadcastErr := &broadcastError.BroadcastingError{}

		matchBroadcastingErr := errors.Is(broadcastErr, anotherBroadcastErr)

		// when & then:
		assert.True(t, matchBroadcastingErr, "should match BroadcastingError type")
	})

	t.Run("should return false for nil target", func(t *testing.T) {
		// given:
		broadcastErr := &broadcastError.BroadcastingError{}

		// when & then:
		assert.False(t, errors.Is(broadcastErr, nil), "should return false for nil target")
	})
}

func TestBroadcastingErrorAs(t *testing.T) {
	t.Run("should support errors.As for type extraction", func(t *testing.T) {
		// given:
		originalErr := &broadcastError.BroadcastingError{
			TxID:      "test-tx-id",
			Reference: "test-ref",
			Operation: broadcastError.CreateAction,
		}
		wrappedErr := fmt.Errorf("wrapped: %w", originalErr)

		// when:
		var extractedErr *broadcastError.BroadcastingError
		found := errors.As(wrappedErr, &extractedErr)

		// then:
		require.True(t, found, "errors.As should find BroadcastingError")
		require.Equal(t, originalErr, extractedErr, "Extracted error should match original")
	})
}

func TestNewCreateActionBroadcastError(t *testing.T) {
	t.Run("should create error with all createAction context", func(t *testing.T) {
		// given:
		underlyingErr := fmt.Errorf("validation failed")
		txID := "abc123def456"
		reference := "ref-789"
		tx := []byte{0x01, 0x02, 0x03}
		noSendChange := []wdk.OutPoint{
			{TxID: "change-tx", Vout: 0},
		}
		processResult := &wdk.ProcessActionResult{
			SendWithResults: []wdk.SendWithResult{
				{TxID: "tx1", Status: wdk.SendWithResultStatusFailed},
			},
			NotDelayedResults: []wdk.ReviewActionResult{
				{TxID: "tx1", Status: wdk.ReviewActionResultStatusDoubleSpend},
			},
		}

		// when:
		broadcastErr := broadcastError.NewCreateActionBroadcastError(
			underlyingErr, txID, reference, tx, noSendChange, processResult)

		// then:
		assert.Equal(t, broadcastError.CreateAction, broadcastErr.Operation)
		assert.Equal(t, underlyingErr, broadcastErr.Err)
		assert.Equal(t, txID, broadcastErr.TxID)
		assert.Equal(t, reference, broadcastErr.Reference)
		assert.Equal(t, tx, broadcastErr.Tx)
		assert.Equal(t, noSendChange, broadcastErr.NoSendChange)
		assert.Equal(t, processResult.SendWithResults, broadcastErr.SendWithResults)
		assert.Equal(t, processResult.NotDelayedResults, broadcastErr.ReviewResults)
	})

	t.Run("should handle nil processResult gracefully", func(t *testing.T) {
		// given:
		underlyingErr := fmt.Errorf("some error")

		// when:
		broadcastErr := broadcastError.NewCreateActionBroadcastError(
			underlyingErr, "tx-id", "ref", []byte{}, nil, nil)

		// then:
		assert.Equal(t, broadcastError.CreateAction, broadcastErr.Operation)
		assert.Equal(t, underlyingErr, broadcastErr.Err)
		assert.Nil(t, broadcastErr.SendWithResults)
		assert.Nil(t, broadcastErr.ReviewResults)
	})
}

func TestNewValidationBroadcastError(t *testing.T) {
	t.Run("should create error with ProcessActionResult context", func(t *testing.T) {
		// given:
		processResult := &wdk.ProcessActionResult{
			SendWithResults: []wdk.SendWithResult{
				{TxID: "primary-tx-id", Status: wdk.SendWithResultStatusFailed},
				{TxID: "secondary-tx", Status: wdk.SendWithResultStatusUnproven},
			},
			NotDelayedResults: []wdk.ReviewActionResult{
				{TxID: "primary-tx-id", Status: wdk.ReviewActionResultStatusServiceError},
			},
		}

		// when:
		broadcastErr := broadcastError.NewValidationBroadcastError(processResult)

		// then:
		assert.Equal(t, broadcastError.ProcessAction, broadcastErr.Operation)
		assert.Contains(t, broadcastErr.Err.Error(), "undelayed result require review")
		assert.Equal(t, "primary-tx-id", broadcastErr.TxID, "should use first SendWithResult TxID")
		assert.Equal(t, processResult.SendWithResults, broadcastErr.SendWithResults)
		assert.Equal(t, processResult.NotDelayedResults, broadcastErr.ReviewResults)
	})

	t.Run("should handle empty SendWithResults", func(t *testing.T) {
		// given:
		processResult := &wdk.ProcessActionResult{
			SendWithResults: []wdk.SendWithResult{},
			NotDelayedResults: []wdk.ReviewActionResult{
				{TxID: "review-tx", Status: wdk.ReviewActionResultStatusInvalidTx},
			},
		}

		// when:
		broadcastErr := broadcastError.NewValidationBroadcastError(processResult)

		// then:
		assert.Equal(t, broadcastError.ProcessAction, broadcastErr.Operation)
		assert.Empty(t, broadcastErr.TxID, "TxID should be empty when no SendWithResults")
		assert.Equal(t, processResult.NotDelayedResults, broadcastErr.ReviewResults)
	})
}

func TestBroadcastingErrorErrorMessageTransactionCounting(t *testing.T) {
	tests := []struct {
		name            string
		sendWithResults []wdk.SendWithResult
		expectedMessage string
	}{
		{
			name: "mixed success and failure",
			sendWithResults: []wdk.SendWithResult{
				{Status: wdk.SendWithResultStatusUnproven},
				{Status: wdk.SendWithResultStatusFailed},
				{Status: wdk.SendWithResultStatusUnproven},
				{Status: wdk.SendWithResultStatusSending},
			},
			expectedMessage: "transactions: 4 total, 2 succeeded, 1 failed",
		},
		{
			name: "all successful",
			sendWithResults: []wdk.SendWithResult{
				{Status: wdk.SendWithResultStatusUnproven},
				{Status: wdk.SendWithResultStatusUnproven},
			},
			expectedMessage: "transactions: 2 total, 2 succeeded, 0 failed",
		},
		{
			name: "all failed",
			sendWithResults: []wdk.SendWithResult{
				{Status: wdk.SendWithResultStatusFailed},
				{Status: wdk.SendWithResultStatusFailed},
			},
			expectedMessage: "transactions: 2 total, 0 succeeded, 2 failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given:
			broadcastErr := &broadcastError.BroadcastingError{
				Operation:       broadcastError.ImmediateBroadcast,
				SendWithResults: tt.sendWithResults,
			}

			// when:
			errorMessage := broadcastErr.Error()

			// then:
			assert.Contains(t, errorMessage, tt.expectedMessage)
		})
	}
}
