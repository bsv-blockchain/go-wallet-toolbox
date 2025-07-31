package testabilities

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func IsNotMockTransportResponderError(t *testing.T, err error) {
	t.Helper()
	require.NotErrorIs(t, err, errors.New("no responder found"))
}
