package primitives

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// BEEF An array of integers, each ranging from 0 to 255, indicating transaction data in BEEF (BRC-62) format.
type BEEF []byte

// MarshalJSON marshals the BEEF to a JSON array of numbers, matching the
// wire format the TS wallet-toolbox expects.
func (b BEEF) MarshalJSON() ([]byte, error) {
	return ExplicitByteArray(b).MarshalJSON()
}

// UnmarshalJSON accepts either a JSON array of numbers or a base64 string.
func (b *BEEF) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("invalid BEEF string: %w", err)
		}
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return fmt.Errorf("invalid BEEF base64: %w", err)
		}
		*b = decoded
		return nil
	}
	var nums []byte
	if err := unmarshalByteNumbers(data, &nums); err != nil {
		return err
	}
	*b = nums
	return nil
}

// unmarshalByteNumbers decodes a JSON array of 0..255 numbers into bytes.
func unmarshalByteNumbers(data []byte, out *[]byte) error {
	var nums []uint16
	if err := json.Unmarshal(data, &nums); err != nil {
		return fmt.Errorf("invalid byte array: %w", err)
	}
	result := make([]byte, len(nums))
	for i, n := range nums {
		if n > 255 {
			return fmt.Errorf("byte array value %d out of range at index %d", n, i)
		}
		result[i] = byte(n)
	}
	*out = result
	return nil
}
