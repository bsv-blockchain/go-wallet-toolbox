package assembler_test

import (
	"encoding/json"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/tsgenerated"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestSequenceNumberDefaultsToMaxUint32(t *testing.T) {
	// given:
	keyDeriver := givenKeyDeriver(t, testusers.Alice)

	// Create a storage result with at least one input. We use the generated one for convenience.
	var createActionResult wdk.StorageCreateActionResult
	err := json.Unmarshal([]byte(tsgenerated.CreateActionResultJSON()), &createActionResult)
	require.NoError(t, err)

	// provide args inputs so that it goes through the user-provided args path
	// The generated JSON has an input at Vin=0. We'll match its Txid and Outpoint.
	storageInput := createActionResult.Inputs[0]

	hash, _ := chainhash.NewHashFromHex(storageInput.SourceTxID)

	providedInputs := []sdk.CreateActionInput{
		{
			Outpoint: transaction.Outpoint{
				Txid:  *hash,
				Index: storageInput.SourceVout,
			},
			SequenceNumber: nil, // This should default to 0xffffffff
		},
	}

	// when:
	assembled, err := assembler.NewCreateActionTransactionAssembler(keyDeriver, providedInputs, &createActionResult).Assemble()

	// then:
	require.NoError(t, err)

	// verify that the input sequence number defaulted correctly
	require.Equal(t, transaction.DefaultSequenceNumber, assembled.Transaction.Inputs[0].SequenceNumber)
}
