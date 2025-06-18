package utils

import (
	"fmt"
	"io"
	"net/http"
)

func WocAPIGetBeefForTX(network, txid string) (string, error) {
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/tx/%s/beef", network, txid)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch from WhatsonChain API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	beefHex := string(body)
	if beefHex == "" {
		return "", fmt.Errorf("empty response received from WhatsonChain API")
	}

	return beefHex, nil
}
