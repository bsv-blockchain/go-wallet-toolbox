package wdk

import (
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"
)

// DefaultPendingSignActionTTL defines the default time-to-live duration for pending sign action requests
const (
	DefaultPendingSignActionTTL = 24 * time.Hour
)

// PendingSignAction represents a structure to hold a transaction and its associated creation arguments before signature.
type PendingSignAction struct {
	Tx               transaction.Transaction
	InputBEEF        *transaction.Beef
	CreateActionArgs ValidCreateActionArgs
}

// PendingSignActionsCache defines an interface for managing cached pending sign actions.
// It allows setting, getting, and deleting actions based on a string reference.
type PendingSignActionsCache interface {
	Set(reference string, action *PendingSignAction) error
	Get(reference string) (*PendingSignAction, error)
	Delete(reference string) error
}
