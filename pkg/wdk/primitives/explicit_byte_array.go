package primitives

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ExplicitByteArray is a byte array, json-serialized to an explicit array of [0..255] numbers.
// Overloads default JSON serialization to a base64 string.
//
// Note: MarshalJSON uses a value receiver (required by json.Marshaler when the
// value type appears non-addressably in encoding paths) while UnmarshalJSON must
// use a pointer receiver to mutate the underlying slice. Both forms are necessary
// for correct JSON round-tripping.
type ExplicitByteArray []byte

// MarshalJSON marshals the byte array to a JSON array of numbers
func (b ExplicitByteArray) MarshalJSON() ([]byte, error) {
	if len(b) == 0 {
		return []byte("[]"), nil
	}

	// Pre-allocate buffer with estimated size
	// Each byte could take up to 3 digits (0-255), plus comma and brackets
	result := make([]byte, 0, len(b)*4+2)

	// Start JSON array
	result = append(result, '[')

	// Append each byte value as a number
	for i, v := range b {
		if i > 0 {
			result = append(result, ',')
		}

		// Convert byte to decimal ASCII representation
		if v < 10 {
			result = append(result, '0'+v)
		} else if v < 100 {
			result = append(result, '0'+v/10, '0'+v%10)
		} else {
			result = append(result, '0'+v/100, '0'+(v/10)%10, '0'+v%10)
		}
	}

	// Close JSON array
	result = append(result, ']')

	return result, nil
}

// UnmarshalJSON parses an ExplicitByteArray from a JSON array of numbers.
// Accepts the matching MarshalJSON output (explicit [0..255] array) and is
// tolerant of `null` (decodes to nil).
func (b *ExplicitByteArray) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*b = nil
		return nil
	}
	var nums []uint16
	if err := json.Unmarshal(data, &nums); err != nil {
		return fmt.Errorf("ExplicitByteArray: %w", err)
	}
	out := make([]byte, len(nums))
	for i, n := range nums {
		if n > 255 {
			return fmt.Errorf("ExplicitByteArray: byte value %d out of range [0,255] at index %d", n, i)
		}
		out[i] = byte(n)
	}
	*b = out
	return nil
}

// Hex returns the hexadecimal representation of the byte array.
func (b ExplicitByteArray) Hex() string {
	return hex.EncodeToString(b)
}
