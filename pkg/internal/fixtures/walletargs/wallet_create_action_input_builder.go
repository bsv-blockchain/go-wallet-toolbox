package walletargs

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	testTx "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

type CreateActionInputSource interface {
	InputBEEFBytes() []byte
	CreateActionInput() sdk.CreateActionInput
}

type CreateActionInputBuilder interface {
	WithDescription(description string) CreateActionInputBuilder
	WithSatoshis(satoshis int) CreateActionInputBuilder
	CreateActionInputSource
}

func NewCreateActionInputBuilder(t testing.TB, user testusers.User) CreateActionInputBuilder {
	return &createActionInputBuilder{
		TB:          t,
		description: "self provided input from tests",
		satoshis:    1,
		user:        user,
	}
}

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
	b.satoshis = must.ConvertToUInt64(satoshis)
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

func (b *createActionInputBuilder) createUnlockingScript(_ *transaction.Transaction) *script.Script {
	unlockingScript := &script.Script{}
	err := unlockingScript.AppendOpcodes(script.Op3)
	require.NoError(b, err, "invalid test setup, cannot create custom unlocking script")

	return unlockingScript
}

func (b *createActionInputBuilder) createInputTx() *transaction.Transaction {
	lockingScript := &script.Script{}
	err := lockingScript.AppendOpcodes(script.Op3, script.OpEQUAL)
	require.NoError(b, err, "invalid test setup, cannot create custom locking script")

	inputTx := testTx.GivenTX().WithInput(b.satoshis+1).WithOutputScript(b.satoshis, lockingScript).TX()
	return inputTx
}
