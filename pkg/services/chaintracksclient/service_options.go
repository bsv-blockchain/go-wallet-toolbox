package chaintracksclient

import (
	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	p2p "github.com/bsv-blockchain/go-teranode-p2p-client"
)

// Option is a functional option for configuring the Adapter.
type Option func(*Adapter)

// WithChaintracks allows injecting a custom chaintracks implementation (useful for testing).
func WithChaintracks(ct chaintracks.Chaintracks) Option {
	return func(a *Adapter) {
		a.ct = ct
	}
}

// WithP2PClient allows injecting a custom P2P client.
func WithP2PClient(client *p2p.Client) Option {
	return func(a *Adapter) {
		a.p2pClient = client
	}
}
