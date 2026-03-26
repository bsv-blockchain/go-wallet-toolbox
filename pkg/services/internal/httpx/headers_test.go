package httpx_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
)

func TestNewHeaders(t *testing.T) {
	headers := httpx.NewHeaders()
	assert.NotNil(t, headers)
	assert.Empty(t, headers.All())
}

func TestHeaders(t *testing.T) {
	testCases := map[string]struct {
		getHeaderSetter func(headers httpx.Headers) httpx.HeaderValueSetter
		key             string
		value           string
	}{
		"Accept": {
			getHeaderSetter: func(headers httpx.Headers) httpx.HeaderValueSetter {
				return headers.Accept()
			},
			key:   "Accept",
			value: "text/plain",
		},
		"Authorization": {
			getHeaderSetter: func(headers httpx.Headers) httpx.HeaderValueSetter {
				return headers.Authorization()
			},
			key:   "Authorization",
			value: "some_token",
		},
		"Content-Type": {
			getHeaderSetter: func(headers httpx.Headers) httpx.HeaderValueSetter {
				return headers.ContentType()
			},
			key:   "Content-Type",
			value: "application/json",
		},
		"User-Agent": {
			getHeaderSetter: func(headers httpx.Headers) httpx.HeaderValueSetter {
				return headers.UserAgent()
			},
			key:   "User-Agent",
			value: "MyApp/1.0",
		},
		"Custom-Header": {
			getHeaderSetter: func(headers httpx.Headers) httpx.HeaderValueSetter {
				return headers.Set("X-Custom-Header")
			},
			key:   "X-Custom-Header",
			value: "custom-value",
		},
	}
	for name, test := range testCases {
		t.Run(name+" set value", func(t *testing.T) {
			headers := httpx.NewHeaders()
			headers = test.getHeaderSetter(headers).Value(test.value)
			assert.Equal(t, map[string]string{test.key: test.value}, headers.All())
		})

		t.Run(name+" set if not empty (use value)", func(t *testing.T) {
			headers := httpx.NewHeaders()
			headers = test.getHeaderSetter(headers).IfNotEmpty(test.value)
			assert.Equal(t, map[string]string{test.key: test.value}, headers.All())
		})

		t.Run(name+" set if not empty (empty value)", func(t *testing.T) {
			headers := httpx.NewHeaders()
			headers = test.getHeaderSetter(headers).IfNotEmpty("")
			assert.Empty(t, headers.All())
		})

		t.Run(name+" set or default (use value)", func(t *testing.T) {
			headers := httpx.NewHeaders()
			headers = test.getHeaderSetter(headers).OrDefault(test.value, "default")
			assert.Equal(t, map[string]string{test.key: test.value}, headers.All())
		})

		t.Run(name+" set or default (use default)", func(t *testing.T) {
			headers := httpx.NewHeaders()
			headers = test.getHeaderSetter(headers).OrDefault("", "default")
			assert.Equal(t, map[string]string{test.key: "default"}, headers.All())
		})
	}

	shortSetters := map[string]struct {
		getHeaderSetter func(headers httpx.Headers) httpx.Headers
		expectedKey     string
		expectedValue   string
	}{
		"AcceptJSON": {
			getHeaderSetter: func(headers httpx.Headers) httpx.Headers {
				return headers.AcceptJSON()
			},
			expectedKey:   "Accept",
			expectedValue: "application/json",
		},
		"ContentTypeJSON": {
			getHeaderSetter: func(headers httpx.Headers) httpx.Headers {
				return headers.ContentTypeJSON()
			},
			expectedKey:   "Content-Type",
			expectedValue: "application/json",
		},
		"AuthorizationBearer": {
			getHeaderSetter: func(headers httpx.Headers) httpx.Headers {
				return headers.AuthorizationBearer().Value("token123")
			},
			expectedKey:   "Authorization",
			expectedValue: "Bearer token123",
		},
	}
	for name, test := range shortSetters {
		t.Run(name, func(t *testing.T) {
			headers := httpx.NewHeaders()
			headers = test.getHeaderSetter(headers)
			assert.Equal(t, map[string]string{test.expectedKey: test.expectedValue}, headers.All())
		})
	}

	t.Run("Set multiple headers", func(t *testing.T) {
		headers := httpx.NewHeaders()
		headers.AcceptJSON().
			AuthorizationBearer().Value("token123").
			UserAgent().Value("TestApp/1.0").
			Set("X-Custom-Header").Value("custom-value")

		expected := map[string]string{
			"Accept":          "application/json",
			"Authorization":   "Bearer token123",
			"User-Agent":      "TestApp/1.0",
			"X-Custom-Header": "custom-value",
		}

		assert.Equal(t, expected, headers.All())
	})
}
