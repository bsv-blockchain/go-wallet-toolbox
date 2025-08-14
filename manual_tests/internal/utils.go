package internal

import (
	"encoding/base64"
	"fmt"
)

func BytesFromBase64(s string) ([]byte, error) {
	result, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 input: %w", err)
	}
	return result, nil
}
