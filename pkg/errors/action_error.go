package errors

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// TransactionError represents an error occurring during a transaction operation.
// TxID is the transaction identifier associated with the error.
// Cause provides the underlying reason for the error, if available.
// WrongHash indicates the transaction TX is not a valid hash.
type TransactionError struct {
	TxID      chainhash.Hash
	Cause     error
	WrongHash bool
}

// NewTransactionError creates a new TransactionError with the specified transaction ID.
func NewTransactionError(txID chainhash.Hash) *TransactionError {
	return &TransactionError{
		TxID: txID,
	}
}

// NewTransactionErrorFromTxIDHex creates a TransactionError from a given transaction ID hex string.
// If the provided string does not represent a valid transaction ID, it returns a TransactionError with WrongHash set to true.
func NewTransactionErrorFromTxIDHex(txIDStr string) *TransactionError {
	txID, err := chainhash.NewHashFromHex(txIDStr)
	if err != nil {
		return &TransactionError{
			WrongHash: true,
		}
	}

	return NewTransactionError(*txID)
}

// Error returns a string representation of the TransactionError, indicating if the transaction ID is incorrect or displaying the given transaction ID.
func (t *TransactionError) Error() string {
	if t.WrongHash {
		return "transaction error (wrong txID)"
	}
	return fmt.Sprintf("transaction error (txID: %s)", t.TxID)
}

// Wrap associates an underlying error with a TransactionError by setting the Cause field and returns the modified TransactionError.
func (t *TransactionError) Wrap(err error) *TransactionError {
	t.Cause = err
	return t
}

// Unwrap returns the underlying cause of the TransactionError, allowing error chaining and inspection.
func (t *TransactionError) Unwrap() error {
	return t.Cause
}

// Is checks if the target error is of type TransactionError or matches the underlying cause of the TransactionError.
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

// CreateActionError represents an error encountered during the creation of an action, encapsulating a reference and cause.
type CreateActionError struct {
	Reference string
	Cause     error
}

// NewCreateActionError creates a new CreateActionError with the provided reference string.
func NewCreateActionError(reference string) *CreateActionError {
	return &CreateActionError{
		Reference: reference,
	}
}

// Error returns a formatted error message for a CreateActionError, including the associated reference identifier.
func (c *CreateActionError) Error() string {
	return fmt.Sprintf("create action failed (reference: %s)", c.Reference)
}

// ReferenceBytes returns the reference of the CreateActionError as a byte slice.
func (c *CreateActionError) ReferenceBytes() []byte {
	return []byte(c.Reference)
}

// Wrap assigns the provided error as the cause of the CreateActionError and returns the updated instance.
func (c *CreateActionError) Wrap(err error) *CreateActionError {
	c.Cause = err
	return c
}

// Unwrap returns the underlying cause of the CreateActionError, if any.
func (c *CreateActionError) Unwrap() error {
	return c.Cause
}

// Is checks whether the target error matches the CreateActionError or its cause if present.
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

// ProcessActionError represents an error that occurred during processing actions, including send and review results.
type ProcessActionError struct {
	SendWithResults []wdk.SendWithResult
	ReviewResults   []wdk.ReviewActionResult
	Cause           error
}

// NewProcessActionError creates a new ProcessActionError instance from send and review operation results.
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

// Wrap sets the cause of the ProcessActionError and returns the updated error instance.
func (p *ProcessActionError) Wrap(err error) *ProcessActionError {
	p.Cause = err
	return p
}

// Unwrap returns the underlying cause of the ProcessActionError, allowing for error unwrapping.
func (p *ProcessActionError) Unwrap() error {
	return p.Cause
}

// Is checks whether the target error is of the type ProcessActionError or matches the underlying cause.
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
