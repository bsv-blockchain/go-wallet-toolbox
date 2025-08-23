package defs

import (
	"time"

	"github.com/go-softwarelab/common/pkg/must"
)

// FailAbandoned represents a configuration for failing abandoned transactions after a specified minimum age in seconds.
type FailAbandoned struct {
	MinTransactionAgeSeconds uint `mapstructure:"min_transaction_age_seconds"`
}

// DefaultFailAbandoned returns a FailAbandoned configuration with a default minimum transaction age of 5 minutes in seconds.
func DefaultFailAbandoned() FailAbandoned {
	return FailAbandoned{
		MinTransactionAgeSeconds: must.ConvertToUInt((5 * time.Minute).Seconds()),
	}
}
