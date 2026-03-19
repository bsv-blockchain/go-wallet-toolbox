package validate_test

import (
	"testing"

	"github.com/go-softwarelab/common/pkg/to"
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
					BasketName: to.Ptr(wdk.BasketNameForChange),
				},
			},
		},
		"multiple valid outputs": {
			inputs: []*entity.Output{
				{
					ID:         1,
					ProvidedBy: string(wdk.ProvidedByStorage),
					Purpose:    wdk.ChangePurpose,
					BasketName: to.Ptr(wdk.BasketNameForChange),
				},
				{
					ID:         2,
					ProvidedBy: string(wdk.ProvidedByStorage),
					Purpose:    wdk.ChangePurpose,
					BasketName: to.Ptr(wdk.BasketNameForChange),
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
				BasketName: to.Ptr(wdk.BasketNameForChange),
			}},
			expected: "provided by field value doesn't match",
		},

		"'Purpose' field value doesn't match wdk.ChangePurpose value": {
			outputs: []*entity.Output{{
				ID:         5,
				ProvidedBy: string(wdk.ProvidedByStorage),
				Purpose:    "bad-purpose",
				BasketName: to.Ptr(wdk.BasketNameForChange),
			}},
			expected: "purpose field value doesn't match",
		},
		"'BasketName' field value is nil": {
			outputs: []*entity.Output{{
				ID:         5,
				ProvidedBy: string(wdk.ProvidedByStorage),
				Purpose:    wdk.ChangePurpose,
			}},
			expected: "basket name field value is set to nil",
		},
		"'BasketName' field value doesn't match wdk.BasketNameForChange value": {
			outputs: []*entity.Output{{
				ID:         5,
				ProvidedBy: string(wdk.ProvidedByStorage),
				Purpose:    wdk.ChangePurpose,
				BasketName: to.Ptr("bad-basket-name"),
			}},
			expected: "basket name field value doesn't match",
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
