package arcade

import "time"

// SetSSEReadWatchdogTimeoutForTests overrides the SSE read-liveness watchdog timeout.
// Test-only: compiled into the test binary, not part of the exported API.
func (s *Service) SetSSEReadWatchdogTimeoutForTests(d time.Duration) {
	s.sseReadWatchdogTimeout = d
}
