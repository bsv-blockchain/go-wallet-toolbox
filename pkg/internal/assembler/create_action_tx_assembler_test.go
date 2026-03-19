package assembler_test

import (
	"encoding/json"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/tsgenerated"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestTxAssemblerAlignsTsGenerated(t *testing.T) {
	// given:
	keyDeriver := givenKeyDeriver(t, testusers.Alice)

	// and:
	createActionResult := givenTsGeneratedStorageCreateActionResult(t)

	// when:
	assembled, err := assembler.NewCreateActionTransactionAssembler(keyDeriver, nil, createActionResult).Assemble()

	// then:
	require.NoError(t, err)

	// when:
	err = assembled.Sign()

	// then:
	require.NoError(t, err)
	require.Equal(t, tsgenerated.SignedTransaction(t).Hex(), assembled.Hex())
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
