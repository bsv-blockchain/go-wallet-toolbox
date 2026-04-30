package defs

// SynchronizeTxStatuses defines configuration for synchronizing transaction statuses with retry attempts.
// MaxAttempts specifies the maximum number of retry attempts allowed when synchronizing transaction statuses.
// If MaxAttempts is set to 0, it indicates that the synchronization will be attempted indefinitely.
// MaxRebroadcastAttempts specifies the maximum number of full rebroadcast cycles before a transaction is
// marked as invalid. If 0 (default), rebroadcasting continues indefinitely — the safe, recommended setting
// for most deployments. Set a positive value only when operators need a hard circuit breaker.
type SynchronizeTxStatuses struct {
	MaxAttempts            uint64 `mapstructure:"max_attempts"`
	MaxRebroadcastAttempts uint64 `mapstructure:"max_rebroadcast_attempts"`
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
