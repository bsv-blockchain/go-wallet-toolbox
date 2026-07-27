package defs

import "fmt"

// ArcadeServiceName is the service name used in queues/results for the Arcade broadcaster.
const ArcadeServiceName = "Arcade"

// ArcGorillaPoolServiceName is the service name for the GorillaPool ARC failover broadcaster.
const ArcGorillaPoolServiceName = "ARC-GorillaPool"

// ArcadeURL is the default mainnet Arcade instance.
const ArcadeURL = "https://arcade-v2-us-1.bsvblockchain.tech"

// ArcadeTTNURL is the default Arcade instance for the public Teranode Test Net (ttn).
// The same host also serves ChainTracks under /chaintracks (client uses /v2/... paths).
const ArcadeTTNURL = "https://arcade-v2-ttn-us-1.bsvblockchain.tech"

// GorillaPoolArcURL is the default mainnet GorillaPool ARC instance (failover only).
const GorillaPoolArcURL = "https://arc.gorillapool.io"

// ArcadeCircuitBreaker configures when the wallet fails over away from Arcade.
type ArcadeCircuitBreaker struct {
	// FailureThreshold is the number of consecutive transport failures that opens the circuit.
	FailureThreshold uint `mapstructure:"failure_threshold"`
	// HealthProbeIntervalSeconds is how often /health is probed while the circuit is open.
	HealthProbeIntervalSeconds uint `mapstructure:"health_probe_interval_seconds"`
}

// Arcade is the configuration for the Arcade broadcaster (primary broadcast path).
type Arcade struct {
	Enabled bool `mapstructure:"enabled"`
	// URL is the base URL of the Arcade instance.
	URL string `mapstructure:"url"`
	// EventsURL is the base URL for the SSE /events endpoint; defaults to URL.
	EventsURL string `mapstructure:"events_url"`
	// CallbackToken scopes webhooks and the SSE stream to this wallet instance.
	// When empty it is derived from the wallet identity key at wiring time.
	CallbackToken string `mapstructure:"callback_token"`
	// CallbackURL is an optional public webhook endpoint (X-CallbackUrl).
	CallbackURL string `mapstructure:"callback_url"`
	// FullStatusUpdates requests every status transition (X-FullStatusUpdates).
	FullStatusUpdates bool `mapstructure:"full_status_updates"`

	CircuitBreaker ArcadeCircuitBreaker `mapstructure:"circuit_breaker"`
}

// Validate checks the Arcade configuration and defaults EventsURL to URL.
func (a *Arcade) Validate() error {
	if !a.Enabled {
		return nil
	}
	if a.URL == "" {
		return fmt.Errorf("arcade is enabled but url is empty")
	}
	if err := validateExternalCallbackURL(a.CallbackURL); err != nil {
		return fmt.Errorf("invalid callback URL: %w", err)
	}
	if a.EventsURL == "" {
		a.EventsURL = a.URL
	}
	return nil
}
