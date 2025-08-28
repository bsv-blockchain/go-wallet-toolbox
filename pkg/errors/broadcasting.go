package errors

import (
	"encoding/hex"
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
	// Err holds the underlying error that caused the broadcasting failure.
	// This could be network errors, validation errors, or service-specific failures.
	Err error

	// TxID is the hexadecimal string representation of the transaction ID that failed to broadcast.
	// For batch operations, this typically represents the primary transaction or first failed transaction.
	TxID string

	// Reference is the unique identifier for the wallet action or operation that was being processed.
	// This helps correlate broadcast failures back to specific user actions or internal operations.
	Reference string

	// SendWithResults contains the detailed broadcast results for each transaction involved in the operation.
	// Each result includes:
	// - TxID: The transaction identifier
	// - Status: One of "unproven" (success), "sending" (in progress), or "failed"
	SendWithResults []wdk.SendWithResult

	// ReviewResults contains detailed information about transactions that require manual review.
	// These are typically transactions that failed validation or encountered issues like double-spends.
	// Each result includes:
	// - TxID: The transaction identifier
	// - Status: One of "success", "doubleSpend", "serviceError", or "invalidTx"
	// - CompetingTxs: List of competing transaction IDs (for double-spend cases)
	// - CompetingBeef: BEEF data for competing transactions (when available)
	ReviewResults []wdk.ReviewActionResult

	// ServiceErrors maps service names to their specific error responses.
	// This helps identify which broadcast services failed and why, enabling
	// targeted retry logic and service-specific error handling.
	ServiceErrors map[string]error

	// Operation indicates which broadcast operation was being performed when the error occurred.
	// Values include: "backgroundBroadcast", "immediateBroadcast", "delayedBroadcast",
	// "createAction", or "processAction". This helps categorize and handle errors appropriately.
	Operation BroadcastOperation

	// Tx contains the raw transaction bytes (BEEF format) that failed to broadcast.
	// This enables retry attempts and detailed transaction analysis for debugging.
	// Under normal circumstances, this contains hex-encoded transaction bytes.
	// If transaction serialization fails, this field will contain an error message in the format:
	// "<error when serializing transaction beef: %s>" where %s is the specific serialization error.
	// May be empty if transaction data is not available or relevant to the error.
	Tx string

	// NoSendChange contains outpoints that should not be broadcast as part of the change handling.
	// This is relevant for "noSend" operations where certain outputs are intentionally kept local.
	// Each outpoint specifies a transaction ID and output index (vout) to exclude from broadcasting.
	NoSendChange []wdk.OutPoint
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
		sendingCount := 0
		for _, result := range e.SendWithResults {
			switch result.Status {
			case wdk.SendWithResultStatusUnproven:
				successCount++
			case wdk.SendWithResultStatusFailed:
				failedCount++
			case wdk.SendWithResultStatusSending:
				sendingCount++
			}
		}

		statusParts := []string{fmt.Sprintf("%d total", len(e.SendWithResults))}
		if successCount > 0 {
			statusParts = append(statusParts, fmt.Sprintf("%d succeeded", successCount))
		}
		if sendingCount > 0 {
			statusParts = append(statusParts, fmt.Sprintf("%d sending", sendingCount))
		}
		if failedCount > 0 {
			statusParts = append(statusParts, fmt.Sprintf("%d failed", failedCount))
		}

		parts = append(parts, fmt.Sprintf("transactions: %s", strings.Join(statusParts, ", ")))
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
			broadcastErr.Tx = fmt.Sprintf("<error when serializing transaction beef: %s>", beefErr.Error())

		} else {
			broadcastErr.Tx = hex.EncodeToString(beefBytes)
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
		Tx:           hex.EncodeToString(tx),
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
