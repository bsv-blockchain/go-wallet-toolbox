package httpx

import (
	"context"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	defaultRetryCount    = 3
	defaultRetryInterval = 1 * time.Second
)

// RetryOnErrOr5xx is a retry condition that retries on any error or if the response status code is 5xx.
func RetryOnErrOr5xx(r *resty.Response, err error) bool {
	return err != nil || (r != nil && r.StatusCode() >= http.StatusInternalServerError)
}

// retryOnTooManyRequestsStatus retries a request the server answered with 429.
//
// res is nil when the call never produced a response at all - a DNS failure, a
// refused dial, or a context deadline firing mid-flight. resty still evaluates
// every retry condition in that case, and Response.StatusCode guards a nil
// RawResponse but not a nil receiver, so dereferencing res here panicked on
// exactly the transport failures the fallback services exist to absorb: the
// panic surfaced as "service WhatsOnChain has paniced with: runtime error:
// invalid memory address" and took the fallback down while the primaries were
// timing out. There is no status to inspect on a nil response, and errors are
// covered by RetryOnErrOr5xx, so it is not retried here.
func retryOnTooManyRequestsStatus(res *resty.Response, err error) bool {
	return res != nil && res.StatusCode() == http.StatusTooManyRequests
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

// pooledTransport clones http.DefaultTransport with a connection pool sized for
// concurrent service traffic. DefaultTransport keeps only 2 idle connections per
// host (DefaultMaxIdleConnsPerHost), so anything past 2 concurrent requests to
// the same host — e.g. the background broadcaster fanning posts at Arcade —
// dials a fresh TCP connection per request and discards it, adding a handshake
// to every call and churning ephemeral ports.
func pooledTransport() *http.Transport {
	t, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Transport{}
	}
	t = t.Clone()
	t.MaxIdleConns = 512
	t.MaxIdleConnsPerHost = 128
	t.IdleConnTimeout = 90 * time.Second
	return t
}

func NewRestyClientFactory() *RestyClientFactory {
	transport := otelhttp.NewTransport(
		pooledTransport(),
		otelhttp.WithClientTrace(func(ctx context.Context) *httptrace.ClientTrace {
			return otelhttptrace.NewClientTrace(ctx)
		}),
	)

	return &RestyClientFactory{
		base: resty.New().
			SetRetryCount(defaultRetryCount).
			SetRetryWaitTime(defaultRetryInterval).
			SetRetryMaxWaitTime(defaultRetryCount * defaultRetryInterval).
			SetTransport(transport).
			AddRetryCondition(retryOnTooManyRequestsStatus),
	}
}
