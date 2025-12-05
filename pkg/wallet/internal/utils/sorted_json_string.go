package utils

import (
	"fmt"
	"sort"
	"strings"
)

// SortedJSONString is a function that generates a consistent JSON string from a map[string]string by sorting keys.
func SortedJSONString(attributes map[string]string) string {
	if len(attributes) == 0 {
		return "{}"
	}

	// 1. Extract and sort the keys
	keys := make([]string, 0, len(attributes))
	for k := range attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 2. Build the JSON string manually, iterating over the sorted keys
	var parts []string
	for _, k := range keys {
		v := attributes[k]
		part := fmt.Sprintf(`"%s":"%s"`, k, v)
		parts = append(parts, part)
	}

	// 3. Join the parts and wrap in braces {}
	return "{" + strings.Join(parts, ",") + "}"
}
