package txutils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
)

// HashOutputScript hashes a locking script (hex) with SHA-256 and returns the little-endian hex representation.
func HashOutputScript(scriptHex string) (string, error) {
	bytes, err := hex.DecodeString(scriptHex)
	if err != nil {
		return "", fmt.Errorf("invalid script hex: %w", err)
	}

	hash := sha256.Sum256(bytes)
	slices.Reverse(hash[:])

	return hex.EncodeToString(hash[:]), nil
}
