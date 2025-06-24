package primitives

import "encoding/hex"

// ExplicitByteArray is a byte array, json-serialized to an explicit array of [0..255] numbers.
// Overloads default JSON serialization to a base64 string.
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

// Hex returns the hexadecimal representation of the byte array.
func (b ExplicitByteArray) Hex() string {
	return hex.EncodeToString(b)
}
