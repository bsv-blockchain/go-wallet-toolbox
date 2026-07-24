package defs

import (
	"fmt"
	"os"
	"strings"
)

// Runtime service-endpoint configuration for the tstn (Teranode Scaling Test Net) network.
//
// Unlike main, test and ttn, the tstn service endpoints are not public and must not be
// hardcoded in this (public) source tree. They are supplied at runtime through environment
// variables:
//
//	TSTN_ARCADE_URL       Arcade broadcaster / ARC endpoint base. Also the fallback host for
//	                      ChainTracks when TSTN_CHAINTRACKS_URL is unset
//	                      (${TSTN_ARCADE_URL}/chaintracks). The go-chaintracks remote client
//	                      appends /v2/... paths to this base.
//	TSTN_CHAINTRACKS_URL  ChainTracks service base URL (without /v2 suffix).
//
// tstn runs only Arcade (broadcast + merkle proofs) and ChainTracks (headers); there is no
// WhatsOnChain / block-explorer service for tstn, so no WhatsOnChain endpoint is configured and
// the WhatsOnChain-only lookups (raw tx, utxo status, txid status, script-hash history) are not
// available on tstn.
const (
	// EnvTstnArcadeURL is the environment variable holding the tstn Arcade / ARC endpoint base.
	EnvTstnArcadeURL = "TSTN_ARCADE_URL"
	// EnvTstnChaintracksURL is the environment variable holding the tstn ChainTracks endpoint.
	EnvTstnChaintracksURL = "TSTN_CHAINTRACKS_URL"
)

// readEnv returns the trimmed value of the named environment variable, or "" when unset/blank.
func readEnv(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

// TstnArcadeURL returns the Arcade broadcaster / ARC endpoint for tstn from TSTN_ARCADE_URL,
// or "" when the variable is unset.
func TstnArcadeURL() string {
	return readEnv(EnvTstnArcadeURL)
}

// TstnChaintracksURL returns the ChainTracks service base URL for tstn. It falls back to
// ${TSTN_ARCADE_URL}/chaintracks when TSTN_CHAINTRACKS_URL is unset (mirroring the ttn
// layout), and returns an error when neither variable is configured.
//
// The go-chaintracks HTTP client treats this as a base and requests /v2/tip,
// /v2/header/height/{n}, etc. under it — so do not include a trailing /v1 or /v2.
func TstnChaintracksURL() (string, error) {
	if explicit := readEnv(EnvTstnChaintracksURL); explicit != "" {
		return explicit, nil
	}
	if arcade := TstnArcadeURL(); arcade != "" {
		return chaintracksURLFromArcade(arcade), nil
	}
	return "", fmt.Errorf(
		"tstn network requires a ChainTracks URL: set %s (or %s) in the environment",
		EnvTstnChaintracksURL, EnvTstnArcadeURL,
	)
}

// chaintracksURLFromArcade derives the ChainTracks base URL from an Arcade host by
// appending /chaintracks. Public arcade deployments expose the go-chaintracks v2 API
// under {arcade}/chaintracks/v2/...
func chaintracksURLFromArcade(arcadeURL string) string {
	return strings.TrimRight(arcadeURL, "/") + "/chaintracks"
}
