package assembler_test

import (
	"encoding/json"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/tsgenerated"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/require"
)

func TestTxAssemblerAlignsTsGenerated(t *testing.T) {
	// given:
	keyDeriver := givenKeyDeriver(t, testusers.Alice)

	// and:
	createActionResult := givenTsGeneratedStorageCreateActionResult(t)

	// FIXME: Workaround START
	// FIXME: Workaround for the fact that the go-sdk's P2PKH Unlocker can't unlock UTXO based on sourceSatoshis & sourceLockingScript
	// FIXME: For now it requires the whole parent transaction to be set in the input
	// FIXME: It should work after this issue is resolved and applied to this project:
	// FIXME: https://github.com/bsv-blockchain/go-sdk/issues/218
	createActionResult.Inputs[0].SourceTransaction = tsgenerated.ParentTransaction(t).Bytes()
	// FIXME: END

	// when:
	signed, err := assembler.NewCreateActionTransactionAssembler(keyDeriver, nil, createActionResult).Assemble()

	// then:
	require.NoError(t, err)

	// when:
	err = signed.Sign()

	// then:
	require.NoError(t, err)
	require.Equal(t, tsgenerated.SignedTransaction(t).Hex(), signed.Hex())
}

func givenKeyDeriver(t *testing.T, user testusers.User) *sdk.KeyDeriver {
	priv, err := ec.PrivateKeyFromHex(user.PrivKey)
	require.NoError(t, err)

	return sdk.NewKeyDeriver(priv)
}

func givenTsGeneratedStorageCreateActionResult(t *testing.T) *wdk.StorageCreateActionResult {
	var createActionResult wdk.StorageCreateActionResult
	err := json.Unmarshal([]byte(tsgenerated.CreateActionResultJSON()), &createActionResult)
	require.NoError(t, err)

	return &createActionResult
}
