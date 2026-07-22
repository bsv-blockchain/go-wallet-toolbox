package defs_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func TestParseBSVNetworkStr(t *testing.T) {
	validCases := map[string]defs.BSVNetwork{
		"main": defs.NetworkMainnet,
		"MAIN": defs.NetworkMainnet,
		"test": defs.NetworkTestnet,
		"Test": defs.NetworkTestnet,
		"ttn":  defs.NetworkTTN,
		"TTN":  defs.NetworkTTN,
		"tstn": defs.NetworkTSTN,
		"TSTN": defs.NetworkTSTN,
	}

	for input, expected := range validCases {
		t.Run("parses "+input, func(t *testing.T) {
			// when:
			network, err := defs.ParseBSVNetworkStr(input)

			// then:
			require.NoError(t, err)
			require.Equal(t, expected, network)
		})
	}

	invalidCases := []string{"", "mainnet", "testnet", "stn", "regtest", "unknown"}
	for _, input := range invalidCases {
		t.Run("rejects "+input, func(t *testing.T) {
			// when:
			_, err := defs.ParseBSVNetworkStr(input)

			// then:
			require.Error(t, err)
		})
	}
}

func TestBSVNetworkValidate(t *testing.T) {
	t.Run("all supported networks validate", func(t *testing.T) {
		for _, n := range []defs.BSVNetwork{defs.NetworkMainnet, defs.NetworkTestnet, defs.NetworkTTN, defs.NetworkTSTN} {
			require.NoError(t, n.Validate(), "network %q should be valid", n)
		}
	})

	t.Run("unknown network is rejected", func(t *testing.T) {
		err := defs.BSVNetwork("nope").Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported BSV network")
	})
}

func TestBSVNetworkIsTeranode(t *testing.T) {
	require.True(t, defs.NetworkTTN.IsTeranode())
	require.True(t, defs.NetworkTSTN.IsTeranode())
	require.False(t, defs.NetworkMainnet.IsTeranode())
	require.False(t, defs.NetworkTestnet.IsTeranode())
}

func TestBSVNetworkIsTestnetBased(t *testing.T) {
	require.False(t, defs.NetworkMainnet.IsTestnetBased())
	require.True(t, defs.NetworkTestnet.IsTestnetBased())
	require.True(t, defs.NetworkTTN.IsTestnetBased())
	require.True(t, defs.NetworkTSTN.IsTestnetBased())
}
