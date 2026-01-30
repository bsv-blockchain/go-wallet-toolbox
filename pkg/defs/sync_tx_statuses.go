package defs

// SynchronizeTxStatuses defines configuration for synchronizing transaction statuses with retry attempts.
// MaxAttempts specifies the maximum number of retry attempts allowed when synchronizing transaction statuses.
// If MaxAttempts is set to 0, it indicates that the synchronization will be attempted indefinitely.
type SynchronizeTxStatuses struct {
	MaxAttempts            uint64 `mapstructure:"max_attempts"`
	CheckNoSendPeriodHours uint64 `mapstructure:"check_no_send_period_hours"`
	BlocksDelay            uint   `mapstructure:"blocks_delay"`
}

// DefaultSynchronizeTxStatuses returns the default configuration for synchronizing transaction statuses with retries.
func DefaultSynchronizeTxStatuses() SynchronizeTxStatuses {
	return SynchronizeTxStatuses{
		MaxAttempts:            10,
		CheckNoSendPeriodHours: 24,
		BlocksDelay:            1,
	}
}
