package bitails

import (
	"errors"
	"fmt"
	"net/url"
	"path"
)

func classifyBroadcastStatus(err error) (alreadyKnown, doubleSpend bool, note string) {
	if err == nil {
		return false, false, ""
	}
	switch {
	case errors.Is(err, ErrAlreadyKnown):
		return true, false, "Transaction already in mempool"
	case errors.Is(err, ErrMissingInputs):
		return false, true, "Missing inputs (double spend)"
	default:
		return false, false, err.Error()
	}
}

// buildURL joins baseURL with any number of path segments, preserving the
func buildURL(baseURL string, segments ...string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL %q: %w", baseURL, err)
	}
	relativePath := path.Join(segments...)
	u = u.ResolveReference(&url.URL{Path: relativePath})
	return u.String(), nil
}

// /tx/{txid}/status
func txStatusURL(baseURL, txID string) (string, error) {
	return buildURL(baseURL, "tx", txID, "status")
}

// /tx/{txid}/proof/tsc
func tscProofURL(baseURL, txID string) (string, error) {
	return buildURL(baseURL, "tx", txID, "proof", "tsc")
}

// /block/{blockHash}/header
func blockHeaderURL(baseURL, blockHash string) (string, error) {
	return buildURL(baseURL, "block", blockHash, "header")
}

// /tx/broadcast/multi
func broadcastURL(baseURL string) (string, error) {
	return buildURL(baseURL, "tx", "broadcast", "multi")
}
