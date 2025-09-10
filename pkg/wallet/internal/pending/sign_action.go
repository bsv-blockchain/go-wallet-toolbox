package pending

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	DefaultPendingSignActionTTL = 24 * time.Hour
)

type SignAction struct {
	Tx               *assembler.AssembledTransaction
	CreateActionArgs wdk.ValidCreateActionArgs
}

type SignActionsCache interface {
	Set(reference string, action *SignAction) error
	Get(reference string) (*SignAction, error)
	Delete(reference string) error
}
