package whatsonchain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func TestNewRequestLimiter(t *testing.T) {
	tests := map[string]struct {
		requestsPerSecond float64
		expectedLimit     rate.Limit
		expectedBurst     int
	}{
		"zero falls back to the no-API-key WoC limit": {
			requestsPerSecond: 0,
			expectedLimit:     rate.Limit(defs.DefaultWhatsOnChainRequestsPerSecond),
			expectedBurst:     defs.DefaultWhatsOnChainRequestsPerSecond,
		},
		"configured limit for an API key plan": {
			requestsPerSecond: 10,
			expectedLimit:     rate.Limit(10),
			expectedBurst:     10,
		},
		"fractional limit keeps a minimal burst": {
			requestsPerSecond: 0.5,
			expectedLimit:     rate.Limit(0.5),
			expectedBurst:     1,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			limiter := newRequestLimiter(test.requestsPerSecond)

			assert.Equal(t, test.expectedLimit, limiter.Limit())
			assert.Equal(t, test.expectedBurst, limiter.Burst())
		})
	}
}
