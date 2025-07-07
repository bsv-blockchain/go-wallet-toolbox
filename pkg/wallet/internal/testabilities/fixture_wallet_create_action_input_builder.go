package testabilities

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sighash "github.com/bsv-blockchain/go-sdk/transaction/sighash"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	testTx "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

type createActionInputBuilder struct {
	testing.TB
	user        testusers.User
	description string
	satoshis    uint64
}

func (b *createActionInputBuilder) WithDescription(description string) CreateActionInputBuilder {
	b.description = description
	return b
}

func (b *createActionInputBuilder) WithSatoshis(satoshis int) CreateActionInputBuilder {
	b.satoshis = uint64(satoshis)
	return b
}

func (b *createActionInputBuilder) InputBEEFBytes() []byte {
	inputTx := b.createInputTx()
	beef, err := inputTx.BEEF()
	require.NoError(b, err, "Input TX should serialize to BEEF, invalid test setup")
	return beef
}

func (b *createActionInputBuilder) CreateActionInput() sdk.CreateActionInput {
	inputTx := b.createInputTx()

	inputUnlockingScript := b.createUnlockingScript(inputTx)

	return sdk.CreateActionInput{
		Outpoint: transaction.Outpoint{
			Txid:  to.Value(inputTx.TxID()),
			Index: 0,
		},
		InputDescription: "self provided input",
		UnlockingScript:  inputUnlockingScript.Bytes(),
	}
}

func (b *createActionInputBuilder) createUnlockingScript(inputTx *transaction.Transaction) *script.Script {
	txToSign := transaction.NewTransaction()
	unlock, err := p2pkh.Unlock(testusers.Alice.PrivateKey(b), to.Ptr(sighash.NoneForkID))
	require.NoError(b, err, "unlocking script template should be created without error, invalid test setup")
	txToSign.AddInputFromTx(inputTx, 0, unlock)

	err = txToSign.Sign()
	require.NoError(b, err, "Transaction should be signed without error, invalid test setup")

	inputUnlockingScript := txToSign.Inputs[0].UnlockingScript
	return inputUnlockingScript
}

func (b *createActionInputBuilder) createInputTx() *transaction.Transaction {
	lockingScript, err := p2pkh.Lock(b.user.Address(b))
	require.NoError(b, err, "locking script template should be created without error, invalid test setup")

	inputTx := testTx.GivenTX().WithOutputScript(b.satoshis, lockingScript).TX()
	return inputTx
}
