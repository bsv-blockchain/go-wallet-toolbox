package whatsonchain

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/internal/dto"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	wocRoot = "https://api.whatsonchain.com/v1/bsv"
)

func classifyBroadcastStatus(status BroadcastStatus, result *wdk.PostedTxID) (shouldReturnError bool) {
	switch status {
	case StatusSuccess:
		result.Result = wdk.PostedTxIDResultSuccess
	case StatusAlreadyBroadcasted:
		result.Result = wdk.PostedTxIDResultAlreadyKnown
		result.AlreadyKnown = true
	case StatusDoubleSpend:
		result.Result = wdk.PostedTxIDResultDoubleSpend
		result.DoubleSpend = true
		shouldReturnError = true
	case StatusMissingInputs:
		result.Result = wdk.PostedTxIDResultMissingInputs
		result.DoubleSpend = true
		shouldReturnError = true
	case StatusError:
		result.Result = wdk.PostedTxIDResultError
		result.Error = fmt.Errorf("broadcast status error")
		shouldReturnError = true
	default:
		result.Result = wdk.PostedTxIDResultError
		result.Error = fmt.Errorf("unknown broadcast status: %d", status)
		shouldReturnError = true
	}

	return
}

func validateScriptHash(scriptHash string) error {
	if scriptHash == "" {
		return fmt.Errorf("scripthash cannot be empty")
	}

	if len(scriptHash) < 20 {
		return fmt.Errorf("invalid scripthash length: too short (minimum 20 characters)")
	}

	if len(scriptHash) > 66 {
		return fmt.Errorf("invalid scripthash length: too long (maximum 66 characters)")
	}

	_, err := hex.DecodeString(scriptHash)
	if err != nil {
		return fmt.Errorf("invalid scripthash format: %w", err)
	}

	return nil
}

func toScriptHistoryItem(item dto.ScriptHashHistoryItem) wdk.ScriptHistoryItem {
	return wdk.ScriptHistoryItem{
		TxHash: item.TxID,
		Height: item.Height,
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

// MakeBaseURL returns "<wocRoot>/<network>"
func MakeBaseURL(network defs.BSVNetwork) (string, error) {
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

// /script/{scriptHash}/unspent/all
func scriptUnspentAllURL(baseURL, scriptHash string) (string, error) {
	return buildURL(baseURL, "script", scriptHash, "unspent", "all")
}

// /txs/status
func txsStatusURL(baseURL string) (string, error) {
	return buildURL(baseURL, "txs", "status")
}
