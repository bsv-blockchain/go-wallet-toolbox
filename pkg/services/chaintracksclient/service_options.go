package chaintracksclient

import "github.com/bsv-blockchain/go-chaintracks/chaintracks"

// Option is a functional option for configuring the Adapter.
type Option func(*Adapter)

// WithChaintracks allows injecting a custom chaintracks implementation (useful for testing).
func WithChaintracks(ct chaintracks.Chaintracks) Option {
	return func(a *Adapter) {
		a.ct = ct
	}
}
