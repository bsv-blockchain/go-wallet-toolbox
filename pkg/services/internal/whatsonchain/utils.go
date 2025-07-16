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

func classifyBroadcastStatus(status BroadcastStatus, result *wdk.PostedTxID) {
	switch status {
	case StatusSuccess:
		result.Result = wdk.PostedTxIDResultSuccess
	case StatusAlreadyBroadcasted:
		result.Result = wdk.PostedTxIDResultAlreadyKnown
		result.AlreadyKnown = true
	case StatusDoubleSpend:
		result.Result = wdk.PostedTxIDResultDoubleSpend
		result.DoubleSpend = true
	case StatusMissingInputs:
		result.Result = wdk.PostedTxIDResultMissingInputs
		result.DoubleSpend = true
	case StatusError:
		result.Result = wdk.PostedTxIDResultError
		result.Error = fmt.Errorf("broadcast status error")
	default:
		result.Result = wdk.PostedTxIDResultError
		result.Error = fmt.Errorf("unknown broadcast status: %d", status)
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
