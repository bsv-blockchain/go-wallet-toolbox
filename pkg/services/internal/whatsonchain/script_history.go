package whatsonchain

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

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

func (woc *WhatsOnChain) getUnconfirmedScriptHistory(ctx context.Context, scriptHash string) (*wdk.ScriptHistoryResult, error) {
	var history wdk.ScriptHashHistoryResponse
	url := fmt.Sprintf("%s/script/%s/unconfirmed/history", woc.url, scriptHash)

	res, err := woc.httpClient.R().
		SetContext(ctx).
		SetResult(&history).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to get unconfirmed script history: %w", err)
	}

	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d getting unconfirmed script history", res.StatusCode())
	}

	if history.Error != "" {
		return nil, fmt.Errorf("API error: %s", history.Error)
	}

	historyItems := make([]wdk.ScriptHistoryItem, len(history.Result))
	for i, item := range history.Result {
		historyItems[i] = wdk.ScriptHistoryItem{
			TxHash: item.TxID,
			Height: item.Height,
		}
	}

	return &wdk.ScriptHistoryResult{
		Name:       ServiceName,
		ScriptHash: scriptHash,
		History:    historyItems,
	}, nil
}

func (woc *WhatsOnChain) getConfirmedScriptHistory(ctx context.Context, scriptHash string, opts *wdk.GetConfirmedScriptHistoryOpts) (*wdk.ScriptHistoryResult, error) {
	var history wdk.ScriptHashHistoryResponse
	url := fmt.Sprintf("%s/script/%s/confirmed/history", woc.url, scriptHash)

	req := woc.httpClient.R().
		SetContext(ctx).
		SetResult(&history)

	if opts != nil {
		if opts.Order != nil {
			req.SetQueryParam("order", opts.Order.String())
		}
		if opts.Limit != nil {
			req.SetQueryParam("limit", fmt.Sprintf("%d", *opts.Limit))
		}
		if opts.Height != nil {
			req.SetQueryParam("height", fmt.Sprintf("%d", *opts.Height))
		}
		if opts.NextPageToken != nil && *opts.NextPageToken != "" {
			req.SetQueryParam("token", *opts.NextPageToken)
		}
	}

	res, err := req.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get confirmed script history: %w", err)
	}

	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d getting confirmed script history", res.StatusCode())
	}

	if history.Error != "" {
		return nil, fmt.Errorf("API error: %s", history.Error)
	}

	historyItems := make([]wdk.ScriptHistoryItem, len(history.Result))
	for i, item := range history.Result {
		historyItems[i] = wdk.ScriptHistoryItem{
			TxHash: item.TxID,
			Height: item.Height,
		}
	}

	return &wdk.ScriptHistoryResult{
		Name:       ServiceName,
		ScriptHash: scriptHash,
		History:    historyItems,
	}, nil
}

// GetScriptHistory retrieves both confirmed and unconfirmed script history.
func (woc *WhatsOnChain) GetScriptHistory(ctx context.Context, scriptHash string, opts *wdk.GetConfirmedScriptHistoryOpts) (*wdk.ScriptHistoryResult, error) {
	if err := validateScriptHash(scriptHash); err != nil {
		return nil, err
	}

	confirmedHistory, err := woc.getConfirmedScriptHistory(ctx, scriptHash, opts)
	if err != nil {
		return nil, err
	}
	unconfirmedHistory, err := woc.getUnconfirmedScriptHistory(ctx, scriptHash)
	if err != nil {
		return nil, err
	}

	combinedHistory := make([]wdk.ScriptHistoryItem, 0, len(confirmedHistory.History)+len(unconfirmedHistory.History))
	combinedHistory = append(combinedHistory, confirmedHistory.History...)
	combinedHistory = append(combinedHistory, unconfirmedHistory.History...)

	return &wdk.ScriptHistoryResult{
		Name:       ServiceName,
		ScriptHash: scriptHash,
		History:    combinedHistory,
	}, nil
}
