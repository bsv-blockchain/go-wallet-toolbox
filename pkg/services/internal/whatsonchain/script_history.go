package whatsonchain

import (
	"context"
	"fmt"
	"iter"
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/internal/dto"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/seq"
)

func (woc *WhatsOnChain) getUnconfirmedScriptHistory(ctx context.Context, scriptHash string) (iter.Seq[wdk.ScriptHistoryItem], error) {
	var history dto.ScriptHashHistoryResponse
	url := fmt.Sprintf("%s/script/%s/unconfirmed/history", woc.url, scriptHash)
	res, err := woc.httpClient.
		R().
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

	historyItems := seq.Map(seq.FromSlice(history.Result), toScriptHistoryItem)
	return historyItems, nil
}

func (woc *WhatsOnChain) getConfirmedScriptHistory(ctx context.Context, scriptHash string) (iter.Seq[wdk.ScriptHistoryItem], error) {
	var history dto.ScriptHashHistoryResponse
	url := fmt.Sprintf("%s/script/%s/confirmed/history", woc.url, scriptHash)
	res, err := woc.httpClient.
		R().
		SetContext(ctx).
		SetResult(&history).
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get confirmed script history: %w", err)
	}

	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d getting confirmed script history", res.StatusCode())
	}

	if history.Error != "" {
		return nil, fmt.Errorf("API error: %s", history.Error)
	}

	historyItems := seq.Map(seq.FromSlice(history.Result), toScriptHistoryItem)
	return historyItems, nil
}

// GetScriptHashHistory retrieves both confirmed and unconfirmed script history.
func (woc *WhatsOnChain) GetScriptHashHistory(ctx context.Context, scriptHash string) (*wdk.ScriptHistoryResult, error) {
	if err := validateScriptHash(scriptHash); err != nil {
		return nil, err
	}

	confirmedHistory, err := woc.getConfirmedScriptHistory(ctx, scriptHash)
	if err != nil {
		return nil, err
	}
	unconfirmedHistory, err := woc.getUnconfirmedScriptHistory(ctx, scriptHash)
	if err != nil {
		return nil, err
	}

	combinedHistory := seq.Collect(seq.Concat(confirmedHistory, unconfirmedHistory))

	return &wdk.ScriptHistoryResult{
		Name:       ServiceName,
		ScriptHash: scriptHash,
		History:    combinedHistory,
	}, nil
}
