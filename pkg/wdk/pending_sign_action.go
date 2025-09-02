package wdk

import (
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"
)

const (
	DefaultPendingSignActionTTL = 24 * time.Hour
)

type PendingSignAction struct {
	Tx               transaction.Transaction
	CreateActionArgs ValidCreateActionArgs
}

type PendingSignActionsCache interface {
	Set(reference string, action *PendingSignAction) error
	Get(reference string) (*PendingSignAction, error)
	Delete(reference string) error
}
