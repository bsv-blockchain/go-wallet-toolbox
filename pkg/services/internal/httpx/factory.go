package httpx

import (
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	defaultRetryCount    = 3
	defaultRetryInterval = 1 * time.Second
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
	base := r.base
	clone := resty.New()
	clone.SetTransport(base.GetClient().Transport)

	clone.SetDebug(base.Debug)
	clone.SetDisableWarn(base.DisableWarn)

	clone.SetRetryCount(base.RetryCount)
	clone.SetRetryWaitTime(base.RetryWaitTime)
	clone.SetRetryMaxWaitTime(base.RetryMaxWaitTime)
	clone.SetRetryAfter(base.RetryAfter)
	clone.SetRetryResetReaders(base.RetryResetReaders)
	for _, cond := range base.RetryConditions {
		clone.AddRetryCondition(cond)
	}
	for _, hook := range base.RetryHooks {
		clone.AddRetryHook(hook)
	}

	return clone
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
