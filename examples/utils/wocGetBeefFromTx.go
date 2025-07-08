package utils

import (
	"fmt"
	"net/http"

	"github.com/go-resty/resty/v2"
)

// WocAPIGetBeefForTX fetches a beef from the WhatsonChain API
func WocAPIGetBeefForTX(network, txid string) (string, error) {
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/tx/%s/beef", network, txid)

	client := resty.New()
	resp, err := client.R().Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch from WhatsonChain API: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return "", fmt.Errorf("transaction not found: %s", txid)
	}

	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("failed to retrieve successful response from WhatsonChain API. Status: %d", resp.StatusCode())
	}

	beefHex := resp.String()
	if beefHex == "" {
		return "", fmt.Errorf("empty response received from WhatsonChain API")
	}

	return beefHex, nil
}
