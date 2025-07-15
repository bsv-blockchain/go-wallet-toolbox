package whatsonchain

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	wocRoot = "https://api.whatsonchain.com/v1/bsv"
)

func waitOrCancel(ctx context.Context, delay time.Duration, txid string) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("context canceled while waiting for tx %s: %w", txid, ctx.Err())
	case <-time.After(delay):
		return nil
	}
}

func classifyBroadcastStatus(status BroadcastStatus) (wdk.PostedTxIDResultStatus, []string) {
	switch status {
	case StatusSuccess:
		return wdk.PostedTxIDResultSuccess, nil
	case StatusAlreadyBroadcasted:
		return wdk.PostedTxIDResultAlreadyKnown, []string{"Transaction already in mempool"}
	case StatusDoubleSpend:
		return wdk.PostedTxIDResultDoubleSpend, []string{"Double spend detected"}
	case StatusMissingInputs:
		return wdk.PostedTxIDResultMissingInputs, []string{"Missing inputs detected"}
	case StatusError:
		return wdk.PostedTxIDResultError, []string{"Broadcast status error"}
	default:
		return wdk.PostedTxIDResultError, []string{fmt.Sprintf("Unknown error: unexpected BroadcastStatus value '%v'", status)}
	}
}

func containsI(subject string, contains ...string) bool {
	subject = strings.ToLower(subject)
	for _, c := range contains {
		if strings.Contains(subject, strings.ToLower(c)) {
			return true
		}
	}
	return false
}

func firstNonNilError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// buildURL joins baseURL with any number of path segments.
func buildURL(baseURL string, segments ...string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL %q: %w", baseURL, err)
	}

	basePath := strings.TrimSuffix(u.Path, "/")
	fullPath := path.Join(append([]string{basePath}, segments...)...)

	if !strings.HasPrefix(fullPath, "/") {
		fullPath = "/" + fullPath
	}
	u.Path = fullPath

	return u.String(), nil
}

// makeBaseURL returns "<wocRoot>/<network>"
func makeBaseURL(network defs.BSVNetwork) (string, error) {
	return buildURL(wocRoot, string(network))
}

// /block/{height}/header
func blockHeaderURL(baseURL string, height uint32) (string, error) {
	return buildURL(baseURL, "block", fmt.Sprint(height), "header")
}

// /block/{blockHash}/header   (hash, not height)
func blockHeaderByHashURL(baseURL, blockHash string) (string, error) {
	return buildURL(baseURL, "block", blockHash, "header")
}

// /tx/{txid}/proof/tsc
func tscProofURL(baseURL, txid string) (string, error) {
	return buildURL(baseURL, "tx", txid, "proof", "tsc")
}

// /chain/info
func chainInfoURL(baseURL string) (string, error) {
	return buildURL(baseURL, "chain", "info")
}
