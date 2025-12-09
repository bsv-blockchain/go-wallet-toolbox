package mapping

import (
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func MapToOverlayNetwork(chain defs.BSVNetwork) overlay.Network {
	switch chain {
	case defs.NetworkMainnet:
		return overlay.NetworkMainnet
	case defs.NetworkTestnet:
		return overlay.NetworkTestnet
	default:
		return overlay.NetworkTestnet
	}
}
