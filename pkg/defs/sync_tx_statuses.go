package defs

// SynchronizeTxStatuses defines configuration for synchronizing transaction statuses with retry attempts.
// MaxAttempts specifies the maximum number of retry attempts allowed when synchronizing transaction statuses.
// If MaxAttempts is set to 0, it indicates that the synchronization will be attempted indefinitely.
// MaxRebroadcastAttempts limits rebroadcast cycles after proof timeout. If set to 0, rebroadcasts are unlimited.
type SynchronizeTxStatuses struct {
	MaxAttempts            uint64 `mapstructure:"max_attempts"`
	MaxRebroadcastAttempts uint64 `mapstructure:"max_rebroadcast_attempts"`
	CheckNoSendPeriodHours uint64 `mapstructure:"check_no_send_period_hours"`
	BlocksDelay            uint   `mapstructure:"blocks_delay"`
}

// DefaultSynchronizeTxStatuses returns the default configuration for synchronizing transaction statuses with retries.
func DefaultSynchronizeTxStatuses() SynchronizeTxStatuses {
	return SynchronizeTxStatuses{
		MaxAttempts:            100,
		MaxRebroadcastAttempts: 0,
		CheckNoSendPeriodHours: 24,
		BlocksDelay:            1,
	}
}
