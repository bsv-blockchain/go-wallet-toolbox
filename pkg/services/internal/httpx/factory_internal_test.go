package httpx

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A transport-level failure (refused dial, DNS error, deadline mid-flight) leaves
// resty with no Response at all, and it still evaluates every retry condition.
// Dereferencing that nil used to panic, which killed the fallback services on
// exactly the network failures they exist to absorb.
func TestRetryConditionsTolerateNilResponse(t *testing.T) {
	err := errors.New("dial tcp: connection refused")

	assert.NotPanics(t, func() {
		assert.False(t, retryOnTooManyRequestsStatus(nil, err))
	})

	assert.NotPanics(t, func() {
		assert.True(t, RetryOnErrOr5xx(nil, err))
	})
}

func TestRetryOnTooManyRequestsStatus(t *testing.T) {
	tests := map[string]struct {
		response *resty.Response
		err      error
		expected bool
	}{
		"nil response after a transport failure": {
			response: nil,
			err:      errors.New("context deadline exceeded"),
			expected: false,
		},
		"response with no underlying http response": {
			response: &resty.Response{},
			expected: false,
		},
		"429 is retried": {
			response: responseWithStatus(http.StatusTooManyRequests),
			expected: true,
		},
		"200 is not retried": {
			response: responseWithStatus(http.StatusOK),
			expected: false,
		},
		"500 is not retried here - RetryOnErrOr5xx covers it": {
			response: responseWithStatus(http.StatusInternalServerError),
			expected: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, retryOnTooManyRequestsStatus(test.response, test.err))
		})
	}
}

// The factory registers retryOnTooManyRequestsStatus on every client it builds,
// so the nil response has to be survivable through the real wiring, not just in
// a direct call: the panic observed in production came out of resty's own retry
// loop, several frames below the caller.
func TestFactoryClientSurvivesUnreachableHost(t *testing.T) {
	client := NewRestyClientFactory().New()
	// The default 3 retries at 1s apart would dominate the test's runtime; the
	// retry conditions are still evaluated on every attempt.
	client.SetRetryWaitTime(time.Millisecond).SetRetryMaxWaitTime(5 * time.Millisecond)

	var (
		resp *resty.Response
		err  error
	)

	require.NotPanics(t, func() {
		// Port 0 is never listening, so the dial fails before any response exists.
		resp, err = client.R().Get("http://127.0.0.1:0/")
	})

	require.Error(t, err, "an unreachable host must surface as an error")
	assert.Zero(t, resp.StatusCode())
}

func responseWithStatus(status int) *resty.Response {
	return &resty.Response{RawResponse: &http.Response{StatusCode: status}}
}
