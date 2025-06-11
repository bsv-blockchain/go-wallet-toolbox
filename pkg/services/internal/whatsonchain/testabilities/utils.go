package testabilities

import (
	"encoding/hex"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/chainhash"
)

func ProofPath(txID string) string {
	return fmt.Sprintf("/v1/bsv/main/tx/%s/proof/tsc", txID)
}

func BlockHeaderPath(blockHash string) string {
	return fmt.Sprintf("/v1/bsv/main/block/%s/header", blockHash)
}

func RawTxPath(txID string) string {
	return fmt.Sprintf("/v1/bsv/main/tx/%s/hex", txID)
}

func MustHashFromHex(s string) *chainhash.Hash {
	h, err := chainhash.NewHashFromHex(s)
	if err != nil {
		panic(fmt.Sprintf("invalid hash hex: %s", s))
	}
	return h
}

func MustDecodeHex(s string) []byte {
	raw, err := hex.DecodeString(s)
	if err != nil {
		panic(fmt.Sprintf("invalid hex string: %s", s))
	}
	return raw
}
