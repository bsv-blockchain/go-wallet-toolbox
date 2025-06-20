package testabilities

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/chainhash"
)

// MustHashFromHex panics on invalid hash input (useful for fixture data)
func MustHashFromHex(s string) *chainhash.Hash {
	h, err := chainhash.NewHashFromHex(s)
	if err != nil {
		panic(fmt.Sprintf("invalid hash hex: %s", s))
	}
	return h
}
