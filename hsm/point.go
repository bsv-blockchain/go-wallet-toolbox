package hsm

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"math/big"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
)

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// addScalarG returns base + scalar*G. scalar is the BRC-42 HMAC output.
func addScalarG(base *ec.PublicKey, scalar []byte) (*ec.PublicKey, error) {
	if base == nil || base.X == nil || base.Y == nil {
		return nil, fmt.Errorf("base public key is incomplete")
	}
	hx, hy := ec.S256().ScalarBaseMult(scalar)
	x, y := ec.S256().Add(base.X, base.Y, hx, hy)
	pub := &ec.PublicKey{Curve: ec.S256(), X: x, Y: y}
	if !pub.Validate() {
		return nil, fmt.Errorf("derived point is not on the curve")
	}
	return pub, nil
}

// signatureFromRS converts a 64-byte r||s ECDSA signature to the canonical
// low-S DER encoding go-sdk verifies. Securosys CKM_ECDSA returns a fixed
// 64-byte encoding that is not low-S normalised.
func signatureFromRS(raw []byte) (*ec.Signature, error) {
	if len(raw) != 64 {
		return nil, fmt.Errorf("ecdsa signature length %d, want 64", len(raw))
	}
	r := new(big.Int).SetBytes(raw[:32])
	s := new(big.Int).SetBytes(raw[32:])
	n := ec.S256().Params().N
	half := new(big.Int).Rsh(new(big.Int).Set(n), 1)
	if s.Cmp(half) > 0 {
		s = new(big.Int).Sub(n, s)
	}
	parsed, err := ec.ParseDERSignature((&ec.Signature{R: r, S: s}).Serialize())
	if err != nil {
		return nil, fmt.Errorf("canonical der signature: %w", err)
	}
	return parsed, nil
}

// minimalHMACKey matches go-sdk ProtoWallet: the HMAC key is the big-endian
// X coordinate with leading zeros removed, and an all-zero key encodes as 0x00.
func minimalHMACKey(key []byte) []byte {
	trimmed := make([]byte, 0, len(key))
	i := 0
	for i < len(key) && key[i] == 0 {
		i++
	}
	trimmed = append(trimmed, key[i:]...)
	if len(trimmed) == 0 {
		return []byte{0}
	}
	return trimmed
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
