package primitives

import (
	"bytes"
	"encoding/base64"
	"fmt"
)

// EncodeBytes32Base64 encodes a fixed [32]byte value as standard base64 for wire/storage use.
//
// Trailing 0x00 padding bytes are trimmed before encoding so the result matches TypeScript SDK
// Base64String certificate types/serials that may be shorter than 32 decoded bytes
// (e.g. "CommonSource identity" → "Q29tbW9uU291cmNlIGlkZW50aXR5").
//
// If the array is all zeros, the full 32 zero bytes are encoded (avoids empty base64).
func EncodeBytes32Base64(b [32]byte) string {
	trimmed := bytes.TrimRight(b[:], "\x00")
	if len(trimmed) == 0 {
		trimmed = b[:]
	}
	return base64.StdEncoding.EncodeToString(trimmed)
}

// DecodeBytes32Base64 decodes a standard base64 string into a fixed [32]byte value.
//
// Decoded lengths from 0 through 32 are accepted; shorter values are zero-padded on the right
// (matching CertificateTypeFromBase64 / StringBase64.ToArray semantics in go-sdk). Lengths
// greater than 32 are rejected.
func DecodeBytes32Base64(s string) ([32]byte, error) {
	var out [32]byte

	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return out, fmt.Errorf("invalid base64: %w", err)
	}
	if len(decoded) > len(out) {
		return out, fmt.Errorf("decoded length %d exceeds 32 bytes", len(decoded))
	}

	copy(out[:], decoded)
	return out, nil
}
