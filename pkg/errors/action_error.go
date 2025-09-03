package errors

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type TransactionError struct {
	TxID  string
	Cause error
}

func NewTransactionError(txID string) *TransactionError {
	return &TransactionError{
		TxID: txID,
	}
}

func (t *TransactionError) Error() string {
	return fmt.Sprintf("transaction error (txID: %s)", t.TxID)
}

func (t *TransactionError) Wrap(err error) *TransactionError {
	t.Cause = err
	return t
}

func (t *TransactionError) Unwrap() error {
	return t.Cause
}

func (t *TransactionError) Is(target error) bool {
	if target == nil {
		return false
	}

	if _, ok := target.(*TransactionError); ok {
		return true
	}

	if t.Cause != nil {
		return errors.Is(t.Cause, target)
	}

	return false
}

type CreateActionError struct {
	Reference string
	Cause     error
}

func NewCreateActionError(reference string) *CreateActionError {
	return &CreateActionError{
		Reference: reference,
	}
}

func (c *CreateActionError) Error() string {
	return fmt.Sprintf("create action failed (reference: %s)", c.Reference)
}

func (c *CreateActionError) Wrap(err error) *CreateActionError {
	c.Cause = err
	return c
}

func (c *CreateActionError) Unwrap() error {
	return c.Cause
}

func (c *CreateActionError) Is(target error) bool {
	if target == nil {
		return false
	}

	if _, ok := target.(*CreateActionError); ok {
		return true
	}

	if c.Cause != nil {
		return errors.Is(c.Cause, target)
	}

	return false
}

type ProcessActionError struct {
	SendWithResults []wdk.SendWithResult
	ReviewResults   []wdk.ReviewActionResult
	Cause           error
}

func NewProcessActionError(sendWithResults []wdk.SendWithResult, reviewResults []wdk.ReviewActionResult) *ProcessActionError {
	return &ProcessActionError{
		SendWithResults: sendWithResults,
		ReviewResults:   reviewResults,
	}
}

func (p *ProcessActionError) Error() string {
	var parts []string

	baseMsg := "process action failed"
	parts = append(parts, baseMsg)

	if len(p.SendWithResults) > 0 {
		successCount := 0
		failedCount := 0
		sendingCount := 0
		for _, result := range p.SendWithResults {
			switch result.Status {
			case wdk.SendWithResultStatusUnproven:
				successCount++
			case wdk.SendWithResultStatusFailed:
				failedCount++
			case wdk.SendWithResultStatusSending:
				sendingCount++
			}
		}

		statusParts := []string{fmt.Sprintf("%d total", len(p.SendWithResults))}
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

	if len(p.ReviewResults) > 0 {
		reviewCount := len(p.ReviewResults)
		parts = append(parts, fmt.Sprintf("review results: %d require review", reviewCount))
	}

	if p.Cause != nil {
		parts = append(parts, fmt.Sprintf("underlying error: %v", p.Cause))
	}

	return strings.Join(parts, "; ")
}

func (p *ProcessActionError) Wrap(err error) *ProcessActionError {
	p.Cause = err
	return p
}

func (p *ProcessActionError) Unwrap() error {
	return p.Cause
}

func (p *ProcessActionError) Is(target error) bool {
	if target == nil {
		return false
	}

	if _, ok := target.(*ProcessActionError); ok {
		return true
	}

	if p.Cause != nil {
		return errors.Is(p.Cause, target)
	}

	return false
}

////////// OLD

// ActionErrorType defines the type for various operations in the wallet system.
type ActionErrorType string

const (
	// BackgroundBroadcast represents a background broadcasting operation.
	BackgroundBroadcast ActionErrorType = "backgroundBroadcast"

	// ImmediateBroadcast represents an immediate broadcasting operation.
	ImmediateBroadcast ActionErrorType = "immediateBroadcast"

	// CreateAction represents an action to create a new wallet action.
	CreateAction ActionErrorType = "createAction"

	// ProcessAction represents an action to process an existing wallet action.
	ProcessAction ActionErrorType = "processAction"
)

// ActionError represents an error that occurred during transaction broadcasting
type ActionError struct {
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
	Operation ActionErrorType

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

// Error implements the error interface for ActionError
func (e *ActionError) Error() string {
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

// Unwrap implements the errors.Unwrap interface for ActionError
func (e *ActionError) Unwrap() error {
	return e.Err
}

// Is implements the errors.Is interface for ActionError
func (e *ActionError) Is(target error) bool {
	if target == nil {
		return false
	}

	if _, ok := target.(*ActionError); ok {
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
) *ActionError {
	broadcastErr := &ActionError{
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
