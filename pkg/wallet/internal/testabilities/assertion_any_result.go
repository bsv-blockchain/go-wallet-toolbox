package testabilities

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type anyResultAssertion struct {
	testing.TB

	result any
}

func (a *anyResultAssertion) HasError(err error) {
	assert.Nil(a, a.result, "Expect nil result when receiving error")
	require.Error(a, err)
}
