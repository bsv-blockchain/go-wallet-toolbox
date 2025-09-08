package primitives

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// String5to2000Bytes represents a string used for descriptions,
// with a length between 5 and 2000 characters.
type String5to2000Bytes string

// Validate checks if the string is between 5 and 2000 characters long
func (d String5to2000Bytes) Validate() error {
	if len(d) < 5 {
		return fmt.Errorf("at least 5 length")
	}
	if len(d) > 2000 {
		return fmt.Errorf("no more than 2000 length")
	}
	return nil
}

// Base64String is a string in base64 format
type Base64String string

// Validate will check if string is proper based64 encoded string
func (s Base64String) Validate() error {
	// Step 1: Check if the string's length is divisible by 4 (Base64 requirement)
	if len(s)%4 != 0 {
		return fmt.Errorf("base64 string length must be divisible by 4")
	}

	// Step 2: Validate padding
	if strings.HasSuffix(string(s), "===") {
		return fmt.Errorf("invalid base64 padding")
	}

	// Step 3: Check if the string is valid Base64
	_, err := base64.StdEncoding.DecodeString(string(s))
	if err != nil {
		return fmt.Errorf("invalid base64 string")
	}

	return nil
}

// StringUnder50Bytes Represents a certificate field name with a maximum length of 50 characters
type StringUnder50Bytes string

// Validate checks if the string is under 50 length
func (s StringUnder50Bytes) Validate() error {
	if len(s) < 1 {
		return fmt.Errorf("at least 1 length")
	}

	if len(s) > 50 {
		return fmt.Errorf("no more than 50 length")
	}
	return nil
}

// StringUnder300 is a string used for basket names, with a length under 300 bytes
type StringUnder300 string

// Validate checks if the string is under 300 bytes long and not empty
func (b StringUnder300) Validate() error {
	if len(b) > 300 {
		return fmt.Errorf("no more than 300 length")
	}
	if len(b) == 0 {
		return fmt.Errorf("at least 1 length")
	}
	return nil
}

// HexString is a string in hexadecimal format
type HexString string

// String returns the string representation of the HexString
func (h HexString) String() string {
	return string(h)
}

var hexRegex = regexp.MustCompile("^[0-9a-fA-F]+$")

// Validate checks if the string is a valid hexadecimal string
func (h HexString) Validate() error {
	if len(h)%2 != 0 {
		return fmt.Errorf("even length, not %d", len(h))
	}

	if !hexRegex.MatchString(string(h)) {
		return fmt.Errorf("hexadecimal string")
	}
	return nil
}

// ToBytes converts the HexString to a byte slice, returning an error if decoding fails.
func (h HexString) ToBytes() ([]byte, error) {
	b, err := hex.DecodeString(string(h))
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex string: %w", err)
	}

	return b, nil
}

// PubKeyHex is a compressed DER secp256k1 public key, exactly 66 hex characters (33 bytes) in length.
type PubKeyHex HexString

const (
	// pubKeyBytesLenCompressed is the length of the compressed pub key
	pubKeyBytesLenCompressed = 33
	// pubKeyBytesLenUncompressed is the length of the uncompressed pub key
	pubKeyBytesLenUncompressed = 65
)

// Validate checks if the string is valid pubkey hexadecimal string
func (pkh PubKeyHex) Validate() error {
	// The public key is stored as a hex string, which means each byte is represented by 2 characters.
	// To get the actual byte length of the public key, we divide the hex string length by 2.
	pkhHalfLen := len(pkh) / 2
	if pkhHalfLen != pubKeyBytesLenCompressed && pkhHalfLen != pubKeyBytesLenUncompressed {
		return fmt.Errorf("invalid pubKey hex length: %d", len(pkh))
	}

	// Validate as HexString
	hs := HexString(pkh)
	if err := hs.Validate(); err != nil {
		return fmt.Errorf("invalid pubKey hex string: %w", err)
	}

	return nil
}

// TXIDHexString is a hexadecimal transaction ID
type TXIDHexString = HexString

// TXIDHexStrings is a collection of hexadecimal transaction IDs.
type TXIDHexStrings []TXIDHexString

// IsEmpty returns true when TXIDHexStrings has no elements.
func (tt TXIDHexStrings) IsEmpty() bool { return len(tt) == 0 }

// ToStringSlice converts the TXIDHexStrings slice into a slice of strings.
func (tt TXIDHexStrings) ToStringSlice() []string {
	slice := make([]string, len(tt))
	for i, t := range tt {
		slice[i] = string(t)
	}
	return slice
}
