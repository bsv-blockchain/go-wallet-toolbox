package utils

import (
	"net/http"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
)

func ConvertNotes(notes []string) wdk.Notes {
	converted := make(wdk.Notes, len(notes))
	for i, note := range notes {
		now := time.Now()
		converted[i] = wdk.ReqHistoryNote{
			When: &now,
			What: note,
		}
	}
	return converted
}

func RetryOnTooManyRequestsStatus(res *resty.Response, err error) bool {
	return res.StatusCode() == http.StatusTooManyRequests
}
