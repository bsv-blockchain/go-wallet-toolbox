package httpx

import (
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	defaultRetryCount    = 2
	defaultRetryInterval = 2 * time.Second
)

// RetryOnErrOr5xx is a retry condition that retries on any error or if the response status code is 5xx.
func RetryOnErrOr5xx(r *resty.Response, err error) bool {
	return err != nil || (r != nil && r.StatusCode() >= http.StatusInternalServerError)
}

func retryOnTooManyRequestsStatus(res *resty.Response, err error) bool {
	return res.StatusCode() == http.StatusTooManyRequests
}

type RestyClientFactory struct {
	base *resty.Client
}

func (r *RestyClientFactory) New() *resty.Client {
	t := r.base.GetClient().Transport
	return resty.New().SetTransport(t)
}

func NewRestyClientFactoryWithBase(base *resty.Client) *RestyClientFactory {
	if base == nil {
		panic("resty client instance is required")
	}
	return &RestyClientFactory{base: base}
}

func NewRestyClientFactory() *RestyClientFactory {
	return &RestyClientFactory{
		base: resty.New().
			SetRetryCount(defaultRetryCount).
			SetRetryWaitTime(defaultRetryInterval).
			SetRetryMaxWaitTime(defaultRetryCount * defaultRetryInterval).
			AddRetryCondition(retryOnTooManyRequestsStatus),
	}
}
