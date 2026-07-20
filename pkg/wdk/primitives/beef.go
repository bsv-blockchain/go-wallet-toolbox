package primitives

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// BEEF An array of integers, each ranging from 0 to 255, indicating transaction data in BEEF (BRC-62) format.
// Overloads default JSON serialization (base64 string) so the wire format matches TypeScript
// storage servers, which expect an explicit [0..255] number array.
type BEEF []byte

// MarshalJSON marshals the BEEF to a JSON array of numbers, matching the
// wire format the TS wallet-toolbox expects.
func (b BEEF) MarshalJSON() ([]byte, error) {
	return ExplicitByteArray(b).MarshalJSON()
}

// UnmarshalJSON accepts either a JSON array of numbers or a base64 string
// (legacy Go encoding/json []byte form). Null decodes to nil.
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

	// Reuse ExplicitByteArray for number-array / null decoding.
	var eba ExplicitByteArray
	if err := eba.UnmarshalJSON(data); err != nil {
		return fmt.Errorf("invalid BEEF: %w", err)
	}
	*b = BEEF(eba)
	return nil
}
