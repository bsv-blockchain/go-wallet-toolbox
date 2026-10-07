package servicequeue_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/servicequeue"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestServiceErrorsLogLevel(t *testing.T) {
	tests := map[string]struct {
		err       error
		wantLevel string
	}{
		"not found":      {fmt.Errorf("tx abc: %w", wdk.ErrNotFoundError), "DEBUG"},
		"canceled":       {fmt.Errorf("call: %w", context.Canceled), "DEBUG"},
		"real failure":   {errors.New("HTTP 429"), "WARN"},
		"deadline error": {context.DeadlineExceeded, "WARN"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			w := &logging.TestWriter{}
			logger := logging.New().WithLevel(defs.LogLevelDebug).WithHandler(defs.TextHandler, w).Logger()
			queue := servicequeue.NewQueue1(logger, "test",
				servicequeue.NewService1("svc", func(context.Context, string) (*string, error) { return nil, test.err }),
			)

			// when:
			_, _ = queue.OneByOne(t.Context(), "arg")

			// then:
			assert.Contains(t, w.String(), "level="+test.wantLevel)
			assert.Contains(t, w.String(), "error when calling service")
		})
	}
}
