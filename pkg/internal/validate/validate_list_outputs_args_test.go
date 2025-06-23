package validate_test

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/validate"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/require"
)

func TestListOutputsArgs(t *testing.T) {
	t.Run("valid args", func(t *testing.T) {
		err := validate.ListOutputsArgs(&wdk.ListOutputsArgs{
			Limit:  10,
			Offset: 0,
		})
		require.NoError(t, err)
	})

	t.Run("invalid txid", func(t *testing.T) {
		err := validate.ListOutputsArgs(&wdk.ListOutputsArgs{
			Limit:      10,
			KnownTxids: []string{"invalidhex"},
		})
		require.Error(t, err)
	})

	t.Run("zero limit", func(t *testing.T) {
		err := validate.ListOutputsArgs(&wdk.ListOutputsArgs{
			Limit: 0,
		})
		require.Error(t, err)
	})
}
