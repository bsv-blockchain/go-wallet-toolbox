package validate_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
)

func TestValidateOriginator(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
	}{
		"exceeds total length limit": {
			originator: strings.Repeat("a", 251),
		},
		"empty part exceeds max part length": {
			originator: "part1." + strings.Repeat("a", 64) + ".part3",
		},
		"contains empty part": {
			originator: "part1..part3",
		},
	}
	for name, test := range errorTestCases {
		t.Run(name, func(t *testing.T) {
			err := validate.Originator(test.originator)
			require.Error(t, err)
		})
	}

	successTestCases := map[string]struct {
		originator string
	}{
		"valid short originator": {
			originator: "short",
		},
		"valid max length originator": {
			originator: strings.Repeat("a", 250),
		},
		"valid originator with multiple parts": {
			originator: "part1.part2.part3",
		},
	}
	for name, test := range successTestCases {
		t.Run(name, func(t *testing.T) {
			err := validate.Originator(test.originator)
			require.NoError(t, err)
		})
	}
}
