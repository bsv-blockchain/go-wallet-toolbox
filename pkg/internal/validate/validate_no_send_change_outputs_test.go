package validate_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoSendChangeOutputs_Success(t *testing.T) {
	validOutput := &entity.Output{
		ID:         1,
		Spendable:  true,
		Change:     false,
		ProvidedBy: string(wdk.ProvidedByStorage),
		Purpose:    wdk.ChangePurpose,
		BasketName: to.Ptr(wdk.BasketNameForChange),
	}

	tests := map[string]struct {
		inputs []*entity.Output
	}{
		"single valid output": {
			inputs: []*entity.Output{validOutput},
		},
		"multiple valid outputs": {
			inputs: []*entity.Output{validOutput, validOutput},
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
		"bad ProvidedBy": {
			outputs: []*entity.Output{{
				ID:         2,
				Spendable:  true,
				Change:     false,
				ProvidedBy: string(wdk.ProvidedByYou),
				Purpose:    "default",
				BasketName: nil,
			}},
			expected: "validate no send change output error:",
		},
		"not spendable": {
			outputs: []*entity.Output{{
				ID:         3,
				Spendable:  false,
				Change:     false,
				ProvidedBy: "storage",
				Purpose:    "default",
				BasketName: nil,
			}},
			expected: "validate no send change output error:",
		},
		"bad Purpose": {
			outputs: []*entity.Output{{
				ID:         4,
				Spendable:  true,
				Change:     false,
				ProvidedBy: "storage",
				Purpose:    "bad-purpose",
				BasketName: nil,
			}},
			expected: "validate no send change output error:",
		},
		"bad BasketName": {
			outputs: []*entity.Output{{
				ID:         5,
				Spendable:  true,
				Change:     false,
				ProvidedBy: "storage",
				Purpose:    "default",
				BasketName: to.Ptr("bad-basket-name"),
			}},
			expected: "validate no send change output error:",
		},
		"nil output": {
			outputs:  []*entity.Output{nil},
			expected: "validate no send change output error:",
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
