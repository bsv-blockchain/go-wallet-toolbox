package testabilities

import (
	_ "embed"
	"encoding/json"
	"sync"
	"testing"
)

//go:embed babbage_cdn_files_info.json
var babbageCDNFilesInfo []byte
var babbageCDNFilesInfoOnce = sync.OnceValue(func() map[string]any {
	var filesInfo map[string]any
	err := json.Unmarshal(babbageCDNFilesInfo, &filesInfo)
	if err != nil {
		panic("failed to unmarshal babbage cdn files info JSON: " + err.Error())
	}

	return filesInfo
})

func BabbageCDNFilesInfo(t testing.TB) map[string]any {
	t.Helper()
	return babbageCDNFilesInfoOnce()
}
