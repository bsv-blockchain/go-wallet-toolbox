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

// buildTxStatusURL constructs a full URL to fetch the status of a transaction.
func buildTxStatusURL(baseURL, txID string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL %q: %w", baseURL, err)
	}
	u.Path = path.Join(u.Path, "tx", txID, "status")
	return u.String(), nil
}
