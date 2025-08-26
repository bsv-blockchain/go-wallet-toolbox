package testabilities

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

type NoSendTransactionFixture struct {
	t              *testing.T
	storageFixture StorageFixture
	user           testusers.User
	activeProvider *storage.Provider
	noSendTxsChain []string
}

func GivenNoSend(t *testing.T, storageFixture StorageFixture, activeProvider *storage.Provider, user testusers.User) *NoSendTransactionFixture {
	return &NoSendTransactionFixture{
		t:              t,
		user:           user,
		storageFixture: storageFixture,
		activeProvider: activeProvider,
	}
}

func (f *NoSendTransactionFixture) FundWallet(satoshis uint64) {
	f.storageFixture.
		Action(f.activeProvider).
		WithSender(f.user).
		WithRecipient(f.user).
		WithSatoshisToInternalize(satoshis).
		WithSatoshisToSend(1).
		Processed()
}

func (f *NoSendTransactionFixture) NoSendTxsHexStrings() []primitives.HexString {
	return slices.Map(f.noSendTxsChain, func(s string) primitives.HexString { return primitives.HexString(s) })
}

func (f *NoSendTransactionFixture) NoSendTxs() []string {
	return f.noSendTxsChain
}

func (f *NoSendTransactionFixture) CreateAction(args wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, *transaction.Transaction) {
	result, err := f.activeProvider.CreateAction(f.t.Context(), f.user.AuthID(), args)
	require.NoError(f.t, err)
	require.NotNil(f.t, result)

	tx, err := assembler.NewCreateActionTransactionAssembler(f.user.KeyDeriver(f.t), nil, result).Assemble()
	require.NoError(f.t, err)
	require.NotNil(f.t, tx)
	require.NoError(f.t, tx.Sign()) // <-- This is important

	return result, tx
}

func (f *NoSendTransactionFixture) ProcessAction(args wdk.ProcessActionArgs) *wdk.ProcessActionResult {
	result, err := f.activeProvider.ProcessAction(f.t.Context(), f.user.AuthID(), args)
	require.NoError(f.t, err)
	require.NotNil(f.t, result)

	return result
}

func (f *NoSendTransactionFixture) CreateAndProcessNoSendAction(prevNoSendOutpoints []wdk.OutPoint) []wdk.OutPoint {
	createActionArgs := fixtures.DefaultValidCreateActionArgs(f.CreateActionNoSendArgsModifier(prevNoSendOutpoints, true))

	createActionResult, signedTx := f.CreateAction(createActionArgs)
	require.NotEmpty(f.t, createActionResult.NoSendChangeOutputVouts)

	txID := signedTx.TxID().String()

	_ = f.ProcessAction(wdk.ProcessActionArgs{
		IsNewTx:   true,
		IsNoSend:  true,
		Reference: to.Ptr(createActionResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:     signedTx.Bytes(),
	})

	noSendOutpoints := slices.Map(createActionResult.NoSendChangeOutputVouts, func(vout int) wdk.OutPoint {
		return wdk.OutPoint{
			TxID: txID,
			Vout: uint32(vout),
		}
	})

	f.noSendTxsChain = append(f.noSendTxsChain, txID)

	return noSendOutpoints
}

func (f *NoSendTransactionFixture) CreateAndProcessSendWithAction(sendWithHexStrings []primitives.HexString, opts ...func(*wdk.ValidCreateActionArgs)) (*wdk.ProcessActionResult, string) {
	createActionArgs := fixtures.DefaultValidCreateActionArgs(opts...)
	createActionResult, tx := f.CreateAction(createActionArgs)
	txID := tx.TxID().String()
	processActionResult := f.ProcessAction(wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsNoSend:   false,
		Reference:  to.Ptr(createActionResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:      tx.Bytes(),
		SendWith:   sendWithHexStrings,
		IsSendWith: true,
	})

	return processActionResult, txID
}

func (f *NoSendTransactionFixture) CreateActionNoSendArgsModifier(prevNoSendOutpoints []wdk.OutPoint, isNoSend bool) func(args *wdk.ValidCreateActionArgs) {
	return func(args *wdk.ValidCreateActionArgs) {
		args.IsNewTx = true
		args.Outputs[0].Satoshis = 1
		args.IsNoSend = isNoSend
		args.Options.NoSend = to.Ptr(primitives.BooleanDefaultFalse(isNoSend))
		args.Options.NoSendChange = prevNoSendOutpoints
	}
}

func (f *NoSendTransactionFixture) CreateActionSendWithArgsModifier(sendWithHexStrings ...primitives.HexString) func(args *wdk.ValidCreateActionArgs) {
	return func(args *wdk.ValidCreateActionArgs) {
		args.IsSendWith = true
		args.Options.SendWith = sendWithHexStrings
	}
}
