package httpx

import (
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	DefaultRetryCount    = 2
	DefaultRetryInterval = 2 * time.Second
)

func RetryOnTooManyRequestsStatus(res *resty.Response, err error) bool {
	return res.StatusCode() == http.StatusTooManyRequests
}
