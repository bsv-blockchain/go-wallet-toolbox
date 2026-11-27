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
	// ProofCheckAfterBlocks is how many blocks must be seen after a broadcast before
	// check_for_proofs polls the tx's merkle path itself, giving Arcade time to push the
	// proof over SSE first. 0 polls on every run. It applies only when Arcade SSE is
	// available; without it every unproven tx is polled on every run, as before.
	ProofCheckAfterBlocks uint `mapstructure:"proof_check_after_blocks"`
}

// EffectiveProofCheckAfterBlocks returns ProofCheckAfterBlocks when proofs arrive over SSE,
// and 0 (poll on every run) otherwise: without SSE, polling is the only source of proofs.
func (s SynchronizeTxStatuses) EffectiveProofCheckAfterBlocks(sseProofs bool) uint {
	if !sseProofs {
		return 0
	}
	return s.ProofCheckAfterBlocks
}

// DefaultSynchronizeTxStatuses returns the default configuration for synchronizing transaction statuses with retries.
func DefaultSynchronizeTxStatuses() SynchronizeTxStatuses {
	return SynchronizeTxStatuses{
		MaxAttempts:            100,
		MaxRebroadcastAttempts: 0,
		CheckNoSendPeriodHours: 24,
		BlocksDelay:            1,
		ProofCheckAfterBlocks:  2,
	}
}
