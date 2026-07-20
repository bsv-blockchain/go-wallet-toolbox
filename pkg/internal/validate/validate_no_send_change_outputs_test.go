package validate_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestNoSendChangeOutputs_Success(t *testing.T) {
	tests := map[string]struct {
		inputs []*entity.Output
	}{
		"single valid output": {
			inputs: []*entity.Output{
				{
					ID:         1,
					ProvidedBy: string(wdk.ProvidedByStorage),
					Purpose:    wdk.ChangePurpose,
				},
			},
		},
		"multiple valid outputs": {
			inputs: []*entity.Output{
				{
					ID:         1,
					ProvidedBy: string(wdk.ProvidedByStorage),
					Purpose:    wdk.ChangePurpose,
				},
				{
					ID:         2,
					ProvidedBy: string(wdk.ProvidedByStorage),
					Purpose:    wdk.ChangePurpose,
				},
			},
		},
		"empty outputs": {
			inputs: []*entity.Output{},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := validate.NoSendChangeOutputs(test.inputs)
			require.NoError(t, err)
		})
	}
}

func TestNoSendChangeOutputs_Error(t *testing.T) {
	tests := map[string]struct {
		outputs  []*entity.Output
		expected string
	}{
		"'ProvidedBy' field value doesn't match wdk.ProvidedByStorage value": {
			outputs: []*entity.Output{{
				ID:         4,
				ProvidedBy: string(wdk.ProvidedByYou),
				Purpose:    wdk.ChangePurpose,
			}},
			expected: "provided by field value doesn't match",
		},

		"'Purpose' field value doesn't match wdk.ChangePurpose value": {
			outputs: []*entity.Output{{
				ID:         5,
				ProvidedBy: string(wdk.ProvidedByStorage),
				Purpose:    "bad-purpose",
			}},
			expected: "purpose field value doesn't match",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := validate.NoSendChangeOutputs(test.outputs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.expected)
		})
	}
}
