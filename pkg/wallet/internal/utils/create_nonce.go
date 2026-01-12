package utils

import (
	"context"
	"encoding/base64"
	"fmt"
	"unicode/utf8"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// BytesToUTF8 converts bytes to a UTF-8 string, mimicking JavaScript's TextDecoder behavior.
// Invalid UTF-8 sequences are replaced with U+FFFD (replacement character), just like TextDecoder does.
func BytesToUTF8(bytes []byte) string {
	result := make([]rune, 0, len(bytes))
	for len(bytes) > 0 {
		r, size := utf8.DecodeRune(bytes)
		result = append(result, r)
		bytes = bytes[size:]
	}
	return string(result)
}

const (
	NonceDataSize  = 16
	NonceHMACSize  = 32
	TotalNonceSize = 48
)

// CreateNonce generates a nonce for authentication and replay protection.
// The nonce consists of 16 random bytes followed by a 32-byte HMAC of those bytes,
// using a key associated with the certifier. The resulting 48-byte nonce is then
// base64-encoded to produce a string-safe representation suitable for transmission
// or storage. The structure is:
//
//	[16 random bytes][32 byte HMAC] -> base64-encoded string (returned as string).
//
// This ensures both uniqueness (random bytes) and integrity/authenticity (HMAC).
func CreateNonce(ctx context.Context, wallet sdk.Interface, randomizer wdk.Randomizer, certifier *ec.PublicKey, originator string) (string, error) {
	firstHalf, err := randomizer.Bytes(NonceDataSize)
	if err != nil {
		return "", fmt.Errorf("failed to generate nonce data bytes: %w", err)
	}
	keyID := BytesToUTF8(firstHalf)

	createHMACResult, err := wallet.CreateHMAC(ctx, sdk.CreateHMACArgs{
		EncryptionArgs: sdk.EncryptionArgs{
			ProtocolID: sdk.Protocol{
				SecurityLevel: sdk.SecurityLevelEveryAppAndCounterparty,
				Protocol:      "server hmac",
			},
			KeyID: keyID,
			Counterparty: sdk.Counterparty{
				Type:         sdk.CounterpartyTypeOther,
				Counterparty: certifier,
			},
		},
		Data: firstHalf,
	}, originator)
	if err != nil {
		return "", fmt.Errorf("failed to create HMAC: %w", err)
	}

	nonce := base64.StdEncoding.EncodeToString(append(firstHalf, createHMACResult.HMAC[:]...))
	return nonce, nil
}
