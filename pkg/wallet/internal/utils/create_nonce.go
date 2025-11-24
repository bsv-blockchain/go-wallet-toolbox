package utils

import (
	"context"
	"encoding/base64"
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

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
//   [16 random bytes][32 byte HMAC] -> base64-encoded string (returned as []byte).
// This ensures both uniqueness (random bytes) and integrity/authenticity (HMAC).
func CreateNonce(ctx context.Context, wallet sdk.Interface, randomizer wdk.Randomizer, certifier *ec.PublicKey, originator string) ([]byte, error) {
	firstHalf, err := randomizer.Bytes(NonceDataSize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce data bytes: %w", err)
	}

	createHMACResult, err := wallet.CreateHMAC(ctx, sdk.CreateHMACArgs{
		EncryptionArgs: sdk.EncryptionArgs{
			ProtocolID: sdk.Protocol{
				SecurityLevel: sdk.SecurityLevelEveryAppAndCounterparty,
				Protocol:      "server hmac",
			},
			KeyID: string(firstHalf),
			Counterparty: sdk.Counterparty{
				Type:         sdk.CounterpartyTypeOther,
				Counterparty: certifier,
			},
		},
		Data: firstHalf,
	}, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to create HMAC: %w", err)
	}

	nonce := base64.StdEncoding.EncodeToString(append(firstHalf, createHMACResult.HMAC[:]...))
	return []byte(nonce), nil
}
