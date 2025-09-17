package pending

import (
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// DefaultPendingSignActionsTTL defines the default time-to-live duration for pending sign action requests
const (
	DefaultPendingSignActionsTTL = 24 * time.Hour
)

// SignAction represents a structure to hold a transaction and its associated creation arguments before signature.
type SignAction struct {
	Tx               transaction.Transaction
	InputBEEF        *transaction.Beef
	CreateActionArgs wdk.ValidCreateActionArgs
}

// SignActionsRepository defines an interface for managing pending sign actions.
// It allows setting, getting, and deleting actions based on a string reference.
type SignActionsRepository interface {
	Save(reference string, action *SignAction) error
	Get(reference string) (*SignAction, error)
	Delete(reference string) error
}
