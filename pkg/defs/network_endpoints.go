package defs

// networkEndpoints holds the per-network default service endpoints resolved by
// DefaultServicesConfig. It centralises the "which services run on which network"
// policy so that spinning up an instance for main/test/ttn/tstn produces a coherent
// broadcaster + header + lookup set.
//
// Summary of the policy:
//
//	network  ARC (merkle)         Arcade (broadcast)   GorillaPool  WhatsOnChain  ChainTracks
//	main     arc.taal.com         arcade-v2-us-1        yes          yes           off (WoC serves headers)
//	test     arc-test.taal.com    off                  no           yes           off
//	ttn      arcade-v2-ttn-us-1   arcade-v2-ttn-us-1   no           yes           on (arcade host /chaintracks/v1)
//	tstn     $TSTN_ARCADE_URL     $TSTN_ARCADE_URL     no           no            on ($TSTN_CHAINTRACKS_URL)
//
// The ChainTracks RemoteURL is populated for every network (derived from the Arcade host)
// even when disabled, so enabling it never points at a stale localhost default.
type networkEndpoints struct {
	arcURL   string
	arcToken string

	arcadeEnabled bool
	arcadeURL     string

	gorillaEnabled bool
	gorillaURL     string

	wocEnabled bool

	chaintracksEnabled bool
	chaintracksURL     string
}

// endpointsForChain returns the default service endpoints for the given network.
//
// For tstn the Arcade / ChainTracks URLs come from the environment (TSTN_ARCADE_URL /
// TSTN_CHAINTRACKS_URL) and are left empty when unset; WalletServices.Validate() then
// surfaces an actionable error pointing at the required environment variables.
func endpointsForChain(chain BSVNetwork) networkEndpoints {
	switch chain {
	case NetworkMainnet:
		return networkEndpoints{
			arcURL:             ArcURL,
			arcToken:           ArcToken,
			arcadeEnabled:      true,
			arcadeURL:          ArcadeURL,
			gorillaEnabled:     true,
			gorillaURL:         GorillaPoolArcURL,
			wocEnabled:         true,
			chaintracksEnabled: false, // WhatsOnChain provides headers on main by default
			chaintracksURL:     chaintracksURLFromArcade(ArcadeURL),
		}
	case NetworkTTN:
		return networkEndpoints{
			// The public ttn Arcade host is ARC-compatible and serves merkle proofs.
			arcURL:             ArcadeTTNURL,
			arcadeEnabled:      true,
			arcadeURL:          ArcadeTTNURL,
			wocEnabled:         true,
			chaintracksEnabled: true,
			chaintracksURL:     chaintracksURLFromArcade(ArcadeTTNURL),
		}
	case NetworkTSTN:
		arcade := TstnArcadeURL()
		// Ignore the "not configured" error here: an empty URL flows into the config and
		// WalletServices.Validate() reports the missing environment variables with context.
		chaintracks, _ := TstnChaintracksURL()
		return networkEndpoints{
			arcURL:             arcade,
			arcadeEnabled:      true,
			arcadeURL:          arcade,
			wocEnabled:         false, // tstn has no WhatsOnChain / block-explorer service
			chaintracksEnabled: true,
			chaintracksURL:     chaintracks,
		}
	case NetworkTestnet:
		fallthrough
	default:
		return networkEndpoints{
			arcURL:             ArcTestURL,
			arcToken:           ArcTestToken,
			arcadeEnabled:      false,
			wocEnabled:         true,
			chaintracksEnabled: false,
			chaintracksURL:     ChaintracksTestURL,
		}
	}
}
