//go:build !(cgo && !nobdk && !ios && !android && (darwin || linux) && (amd64 || arm64))

package signaturebackend_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/signaturebackend"
)

func TestNonCgoBuild_KeepsBuiltIn(t *testing.T) {
	assert.Equal(t, "go-sdk-builtin", signaturebackend.Name)
}
