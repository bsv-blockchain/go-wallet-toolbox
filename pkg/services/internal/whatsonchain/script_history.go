package whatsonchain

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/internal/dto"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/slices"
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

func (woc *WhatsOnChain) getUnconfirmedScriptHistory(ctx context.Context, scriptHash string) ([]wdk.ScriptHistoryItem, error) {
	var history dto.ScriptHashHistoryResponse
	url := fmt.Sprintf("%s/script/%s/unconfirmed/history", woc.url, scriptHash)

	res, err := woc.httpClientForScriptHashHistory.
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

	historyItems := slices.Map(history.Result, toScriptHistoryItem)
	return historyItems, nil
}

func (woc *WhatsOnChain) getConfirmedScriptHistory(ctx context.Context, scriptHash string) ([]wdk.ScriptHistoryItem, error) {
	var history dto.ScriptHashHistoryResponse
	url := fmt.Sprintf("%s/script/%s/confirmed/history", woc.url, scriptHash)

	res, err := woc.httpClientForScriptHashHistory.
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

	historyItems := slices.Map(history.Result, toScriptHistoryItem)
	return historyItems, nil
}

// GetScriptHistory retrieves both confirmed and unconfirmed script history.
func (woc *WhatsOnChain) GetScriptHistory(ctx context.Context, scriptHash string) (*wdk.ScriptHistoryResult, error) {
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

	combinedHistory := seq.Collect(seq.Concat(seq.FromSlice(confirmedHistory), seq.FromSlice(unconfirmedHistory)))

	return &wdk.ScriptHistoryResult{
		Name:       ServiceName,
		ScriptHash: scriptHash,
		History:    combinedHistory,
	}, nil
}

func toScriptHistoryItem(item dto.ScriptHashHistoryItem) wdk.ScriptHistoryItem {
	return wdk.ScriptHistoryItem{
		TxHash: item.TxID,
		Height: item.Height,
	}
}
