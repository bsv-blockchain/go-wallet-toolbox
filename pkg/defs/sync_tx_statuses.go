package defs

// SynchronizeTxStatuses defines configuration for synchronizing transaction statuses with retry attempts.
// MaxAttempts specifies the maximum number of retry attempts allowed when synchronizing transaction statuses.
type SynchronizeTxStatuses struct {
	MaxAttempts uint64 `mapstructure:"max_attempts"`
}

// DefaultSynchronizeTxStatuses returns the default configuration for synchronizing transaction statuses with retries.
func DefaultSynchronizeTxStatuses() SynchronizeTxStatuses {
	return SynchronizeTxStatuses{
		MaxAttempts: 10,
	}
}
