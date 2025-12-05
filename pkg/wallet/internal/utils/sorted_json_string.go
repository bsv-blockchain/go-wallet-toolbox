package utils

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// SortedJSONString generates a consistent JSON string from a map[string]string by sorting keys.
func SortedJSONString(attributes map[string]string) (string, error) {
	if len(attributes) == 0 {
		return "{}", nil
	}

	// 1. Extract and sort the keys
	keys := make([]string, 0, len(attributes))
	for k := range attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 2. Build the JSON string manually, iterating over the sorted keys
	parts := make([]string, len(keys))
	for i, k := range keys {
		keyJSON, err := json.Marshal(k)
		if err != nil {
			return "", fmt.Errorf("failed to marshal key %q: %w", k, err)
		}
		valueJSON, err := json.Marshal(attributes[k])
		if err != nil {
			return "", fmt.Errorf("failed to marshal value for key %q: %w", k, err)
		}
		parts[i] = string(keyJSON) + ":" + string(valueJSON)
	}

	// 3. Join the parts and wrap in braces {}
	return "{" + strings.Join(parts, ",") + "}", nil
}
