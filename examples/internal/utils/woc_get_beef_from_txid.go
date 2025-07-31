package utils

import (
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/go-resty/resty/v2"
)

// WocAPIGetBeefForTX fetches a beef from the WhatsonChain API
func WocAPIGetBeefForTX(network defs.BSVNetwork, txid string) (string, error) {
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

	atomicBeef, err := createAtomicBeef(beefHex)
	if err != nil {
		return "", fmt.Errorf("failed to create atomic beef: %w", err)
	}

	return atomicBeef, nil
}

// createAtomicBeef creates an atomic beef from a beef hex
func createAtomicBeef(beefHex string) (string, error) {
	tx, err := transaction.NewTransactionFromBEEFHex(beefHex)
	if err != nil {
		return "", fmt.Errorf("failed to parse beef: %w", err)
	}

	atomicBeef, err := tx.AtomicBEEF(true)
	if err != nil {
		return "", fmt.Errorf("failed to convert to atomic beef: %w", err)
	}

	return hex.EncodeToString(atomicBeef), nil
}
