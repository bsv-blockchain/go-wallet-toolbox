package defs_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func TestFeeModelValidateAcceptsZero(t *testing.T) {
	// given:
	model := defs.FeeModel{Type: defs.SatPerKB, Value: 0}

	// when:
	err := model.Validate()

	// then:
	require.NoError(t, err)
}

func TestFeeModelValidateRejectsNegative(t *testing.T) {
	// given:
	model := defs.FeeModel{Type: defs.SatPerKB, Value: -1}

	// when:
	err := model.Validate()

	// then:
	require.Error(t, err)
}

func TestFeeModelValidateAcceptsPositive(t *testing.T) {
	// given:
	model := defs.DefaultFeeModel()

	// when:
	err := model.Validate()

	// then:
	require.NoError(t, err)
}
