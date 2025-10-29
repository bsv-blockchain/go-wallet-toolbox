package testabilities

import (
	_ "embed"
	"encoding/json"
	"sync"
	"testing"
)

//go:embed woc_last_10_headers.json
var wocLast10HeadersJSON []byte
var wocLast10HeadersOnce = sync.OnceValue(func() []map[string]any {
	var headers []map[string]any
	err := json.Unmarshal(wocLast10HeadersJSON, &headers)
	if err != nil {
		panic("failed to unmarshal woc last 10 headers JSON: " + err.Error())
	}

	return headers
})

func WOCLast10Headers(t testing.TB) []map[string]any {
	t.Helper()
	return wocLast10HeadersOnce()
}

func WOCLastHeader(t testing.TB) map[string]any {
	t.Helper()
	headers := WOCLast10Headers(t)
	if len(headers) == 0 {
		t.Fatal("no headers available in test data")
	}
	return headers[0]
}
