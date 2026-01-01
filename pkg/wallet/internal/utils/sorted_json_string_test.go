package utils

import (
	"testing"
)

func TestSortedJSONString(t *testing.T) {
	tests := []struct {
		name       string
		attributes map[string]string
		want       string
		wantErr    bool
	}{
		{
			name:       "empty map",
			attributes: map[string]string{},
			want:       "{}",
		},
		{
			name:       "nil map",
			attributes: nil,
			want:       "{}",
		},
		{
			name:       "single key-value pair",
			attributes: map[string]string{"key": "value"},
			want:       `{"key":"value"}`,
		},
		{
			name: "multiple keys sorted alphabetically",
			attributes: map[string]string{
				"zebra": "g",
				"apple": "x",
				"mango": "a",
			},
			want: `{"apple":"x","mango":"a","zebra":"g"}`,
		},
		{
			name: "keys with special characters - quotes",
			attributes: map[string]string{
				`key"with"quotes`: `value"with"quotes`,
			},
			want: `{"key\"with\"quotes":"value\"with\"quotes"}`,
		},
		{
			name: "keys with special characters - backslash",
			attributes: map[string]string{
				`key\with\backslash`: `value\with\backslash`,
			},
			want: `{"key\\with\\backslash":"value\\with\\backslash"}`,
		},
		{
			name: "keys with special characters - newline and tab",
			attributes: map[string]string{
				"key\nwith\nnewline": "value\twith\ttab",
			},
			want: `{"key\nwith\nnewline":"value\twith\ttab"}`,
		},
		{
			name: "empty string key and value",
			attributes: map[string]string{
				"": "",
			},
			want: `{"":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SortedJSONString(tt.attributes)
			if (err != nil) != tt.wantErr {
				t.Errorf("SortedJSONString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SortedJSONString() = %v, want %v", got, tt.want)
			}
		})
	}
}
