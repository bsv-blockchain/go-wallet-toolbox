package errors

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// BroadcastOperation defines the type for various operations in the wallet system.
type BroadcastOperation string

const (
	// BackgroundBroadcast represents a background broadcasting operation.
	BackgroundBroadcast BroadcastOperation = "backgroundBroadcast"

	// ImmediateBroadcast represents an immediate broadcasting operation.
	ImmediateBroadcast BroadcastOperation = "immediateBroadcast"

	// DelayedBroadcast represents a delayed broadcasting operation.
	DelayedBroadcast BroadcastOperation = "delayedBroadcast"

	// CreateAction represents an action to create a new wallet action.
	CreateAction BroadcastOperation = "createAction"

	// ProcessAction represents an action to process an existing wallet action.
	ProcessAction BroadcastOperation = "processAction"
)

// BroadcastingError represents an error that occurred during transaction broadcasting
type BroadcastingError struct {
	Err             error
	TxID            string
	Reference       string
	SendWithResults []wdk.SendWithResult
	ReviewResults   []wdk.ReviewActionResult
	ServiceErrors   map[string]error
	Operation       BroadcastOperation
	Tx              []byte
	NoSendChange    []wdk.OutPoint
}

// Error implements the error interface for BroadcastingError
func (e *BroadcastingError) Error() string {
	var parts []string

	baseMsg := fmt.Sprintf("broadcasting failed during %s", e.Operation)
	if e.TxID != "" {
		baseMsg += fmt.Sprintf(" for txID %s", e.TxID)
	}
	if e.Reference != "" {
		baseMsg += fmt.Sprintf(" (reference: %s)", e.Reference)
	}
	parts = append(parts, baseMsg)

	if len(e.SendWithResults) > 0 {
		successCount := 0
		failedCount := 0
		for _, result := range e.SendWithResults {
			switch result.Status {
			case wdk.SendWithResultStatusUnproven:
				successCount++
			case wdk.SendWithResultStatusFailed:
				failedCount++
			}
		}
		parts = append(parts, fmt.Sprintf("transactions: %d total, %d succeeded, %d failed",
			len(e.SendWithResults), successCount, failedCount))
	}

	if len(e.ServiceErrors) > 0 {
		parts = append(parts, fmt.Sprintf("service errors: %d services failed", len(e.ServiceErrors)))
	}

	if e.Err != nil {
		parts = append(parts, fmt.Sprintf("underlying error: %v", e.Err))
	}

	return strings.Join(parts, "; ")
}

// Unwrap implements the errors.Unwrap interface for BroadcastingError
func (e *BroadcastingError) Unwrap() error {
	return e.Err
}

// Is implements the errors.Is interface for BroadcastingError
func (e *BroadcastingError) Is(target error) bool {
	if target == nil {
		return false
	}

	if _, ok := target.(*BroadcastingError); ok {
		return true
	}

	if e.Err != nil {
		return errors.Is(e.Err, target)
	}

	return false
}

// NewImmediateBroadcastError creates an error for immediate broadcasting failures
func NewImmediateBroadcastError(
	err error,
	txID string,
	beef *transaction.Beef,
	processResult *wdk.ProcessActionResult,
	serviceErrors map[string]error,
	logger *slog.Logger,
) *BroadcastingError {
	broadcastErr := &BroadcastingError{
		Err:           err,
		Operation:     ImmediateBroadcast,
		TxID:          txID,
		ServiceErrors: serviceErrors,
	}

	if processResult != nil {
		broadcastErr.SendWithResults = processResult.SendWithResults
		broadcastErr.ReviewResults = processResult.NotDelayedResults
	}

	if beef != nil {
		if beefBytes, beefErr := beef.Bytes(); beefErr != nil {
			logger.Warn("Failed to serialize BEEF for error context",
				slog.String("error", beefErr.Error()),
				slog.String("txID", txID),
				slog.String("operation", string(ImmediateBroadcast)),
			)
		} else {
			broadcastErr.Tx = beefBytes
		}
	}

	return broadcastErr
}

// NewCreateActionBroadcastError creates an error for createAction broadcasting failures
func NewCreateActionBroadcastError(
	err error,
	txID string,
	reference string,
	tx []byte,
	noSendChange []wdk.OutPoint,
	processResult *wdk.ProcessActionResult,
) *BroadcastingError {
	broadcastErr := &BroadcastingError{
		Err:          err,
		Operation:    CreateAction,
		TxID:         txID,
		Reference:    reference,
		Tx:           tx,
		NoSendChange: noSendChange,
	}

	if processResult != nil {
		broadcastErr.SendWithResults = processResult.SendWithResults
		broadcastErr.ReviewResults = processResult.NotDelayedResults
	}

	return broadcastErr
}

// NewValidationBroadcastError creates an error for validation failures with ProcessActionResult context
func NewValidationBroadcastError(processResult *wdk.ProcessActionResult) *BroadcastingError {
	broadcastErr := &BroadcastingError{
		Err:       fmt.Errorf("undelayed result require review"),
		Operation: ProcessAction,
	}

	if len(processResult.SendWithResults) > 0 {
		broadcastErr.TxID = string(processResult.SendWithResults[0].TxID)
		broadcastErr.SendWithResults = processResult.SendWithResults
	}

	if len(processResult.NotDelayedResults) > 0 {
		broadcastErr.ReviewResults = processResult.NotDelayedResults
	}

	return broadcastErr
}
