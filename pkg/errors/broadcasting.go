package errors

import (
	"errors"
	"fmt"
	"log/slog"

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
	if e.Err == nil {
		return fmt.Sprintf("broadcasting failed during %s for txID %s (reference: %s)",
			e.Operation, e.TxID, e.Reference)
	}
	return fmt.Sprintf("broadcasting failed during %s for txID %s (reference: %s): %v",
		e.Operation, e.TxID, e.Reference, e.Err)
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

// EnhanceWithCreateActionContext enhances the error with context specific to a CreateAction operation
func EnhanceWithCreateActionContext(
	err error,
	txID, reference string,
	tx []byte,
	noSendChange []wdk.OutPoint,
	processResult *wdk.ProcessActionResult,
) error {
	var broadcastErr *BroadcastingError
	if errors.As(err, &broadcastErr) {
		return broadcastErr.
			WithOperation(CreateAction).
			WithReference(reference).
			WithTxIDIfEmpty(txID).
			WithProcessActionContext(processResult, txID, reference).
			WithTransactionData(tx, noSendChange)
	}

	return NewBroadcastingError(err, CreateAction).
		WithTxID(txID).
		WithReference(reference).
		WithTransactionData(tx, noSendChange).
		WithProcessActionContext(processResult, txID, reference)
}

// WithContext adds context values from processAction result
func (e *BroadcastingError) WithContext(processResult *wdk.ProcessActionResult, txID, reference string) *BroadcastingError {
	e.TxID = txID
	e.Reference = reference
	if processResult != nil {
		e.SendWithResults = processResult.SendWithResults
		e.ReviewResults = processResult.NotDelayedResults
	}
	return e
}

// WithTxID is a convenience method to set the TxID and return the error for chaining
func (e *BroadcastingError) WithTxID(txID string) *BroadcastingError {
	e.TxID = txID
	return e
}

// WithPrimaryTxID sets the TxID to the first transaction ID in the slice if available
func (e *BroadcastingError) WithPrimaryTxID(txIDs []string) *BroadcastingError {
	if len(txIDs) > 0 {
		e.TxID = txIDs[0]
	}
	return e
}

// WithTxIDIfEmpty sets the TxID only if it's currently empty
func (e *BroadcastingError) WithTxIDIfEmpty(txID string) *BroadcastingError {
	if e.TxID == "" {
		e.TxID = txID
	}
	return e
}

// WithOperation sets the operation context for the error
func (e *BroadcastingError) WithOperation(operation BroadcastOperation) *BroadcastingError {
	e.Operation = operation
	return e
}

// WithReference sets the reference context for the error
func (e *BroadcastingError) WithReference(reference string) *BroadcastingError {
	e.Reference = reference
	return e
}

// WithTransactionData adds transaction bytes and noSendChange context
func (e *BroadcastingError) WithTransactionData(tx []byte, noSendChange []wdk.OutPoint) *BroadcastingError {
	e.Tx = tx
	e.NoSendChange = noSendChange
	return e
}

// WithBEEFData adds BEEF transaction data, handling errors gracefully
func (e *BroadcastingError) WithBEEFData(log *slog.Logger, beef *transaction.Beef, noSendChange []wdk.OutPoint) *BroadcastingError {
	if beef == nil {
		return e
	}

	beefBytes, err := beef.Bytes()
	if err != nil {
		log.Error("Failed to serialize BEEF for error context",
			slog.String("error", err.Error()),
			slog.String("txID", e.TxID),
			slog.String("operation", string(e.Operation)),
			slog.Any("beef", beef),
		)
		return e
	}

	return e.WithTransactionData(beefBytes, noSendChange)
}

// WithProcessActionContext adds context from processAction result if available
func (e *BroadcastingError) WithProcessActionContext(processResult *wdk.ProcessActionResult, txID, reference string) *BroadcastingError {
	if processResult != nil {
		return e.WithContext(processResult, txID, reference)
	}
	return e
}

// WithPostBEEFResults adds results from PostBEEF operations, if available
func (e *BroadcastingError) WithPostBEEFResults(results PostBEEFResults) *BroadcastingError {
	if results == nil {
		return e
	}
	return e.WithServiceErrors(results.ServiceErrors())
}

// WithSendWithResults adds send results to the error context
func (e *BroadcastingError) WithSendWithResults(results []wdk.SendWithResult) *BroadcastingError {
	e.SendWithResults = results
	return e
}

// PostBEEFResults interface to avoid direct dependency on services package
type PostBEEFResults interface {
	ServiceErrors() map[string]error
}

// WithServiceErrors adds service-specific error information
func (e *BroadcastingError) WithServiceErrors(serviceErrors map[string]error) *BroadcastingError {
	e.ServiceErrors = serviceErrors
	return e
}

// NewBroadcastingError creates a new broadcasting error with the given underlying error
func NewBroadcastingError(err error, operation BroadcastOperation) *BroadcastingError {
	return &BroadcastingError{
		Err:       err,
		Operation: operation,
	}
}
