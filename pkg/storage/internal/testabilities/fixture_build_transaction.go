package testabilities

import (
	"maps"
	stdslices "slices"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

type NoSendTransactionFixture interface {
	FundWallet(satoshis uint64)
	NoSendTxsHexStrings() []primitives.HexString
	NoSendTxs() []string
	LastCreateActionResult() *wdk.StorageCreateActionResult
	LastUsedChangeOutputsCounter() int
	AllRemainedNoSendChange() []wdk.OutPoint
	CreateAction(args wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, *transaction.Transaction)
	WillSendSats(sats uint64) NoSendTransactionFixture
	ProcessAction(args wdk.ProcessActionArgs) *wdk.ProcessActionResult
	CreateAndProcessNoSendAction(prevNoSendOutpoints []wdk.OutPoint) []wdk.OutPoint
	CreateAndProcessSendWithAction(sendWithHexStrings []primitives.HexString, opts ...func(*wdk.ValidCreateActionArgs)) (*wdk.ProcessActionResult, string)
	CreateActionNoSendArgsModifier(prevNoSendOutpoints []wdk.OutPoint, isNoSend bool) func(args *wdk.ValidCreateActionArgs)
	CreateActionSendWithArgsModifier(sendWithHexStrings ...primitives.HexString) func(args *wdk.ValidCreateActionArgs)
}

type noSendTransactionFixture struct {
	t                            *testing.T
	storageFixture               StorageFixture
	user                         testusers.User
	activeProvider               *storage.Provider
	noSendTxsChain               []string
	satsToSend                   primitives.SatoshiValue
	lastCreateActionResult       *wdk.StorageCreateActionResult
	lastUsedChangeOutputsCounter int
	allRemainedNoSendChange      map[wdk.OutPoint]struct{}
}

func GivenNoSend(t *testing.T, storageFixture StorageFixture, activeProvider *storage.Provider, user testusers.User) NoSendTransactionFixture {
	return &noSendTransactionFixture{
		t:                       t,
		user:                    user,
		storageFixture:          storageFixture,
		activeProvider:          activeProvider,
		satsToSend:              1,
		allRemainedNoSendChange: make(map[wdk.OutPoint]struct{}),
	}
}

func (f *noSendTransactionFixture) FundWallet(satoshis uint64) {
	f.storageFixture.
		Action(f.activeProvider).
		WithSender(f.user).
		WithRecipient(f.user).
		WithSatoshisToInternalize(satoshis).
		WithSatoshisToSend(1).
		Processed()
}

func (f *noSendTransactionFixture) NoSendTxsHexStrings() []primitives.HexString {
	return slices.Map(f.noSendTxsChain, func(s string) primitives.HexString { return primitives.HexString(s) })
}

func (f *noSendTransactionFixture) NoSendTxs() []string {
	return f.noSendTxsChain
}

func (f *noSendTransactionFixture) LastCreateActionResult() *wdk.StorageCreateActionResult {
	return f.lastCreateActionResult
}

func (f *noSendTransactionFixture) LastUsedChangeOutputsCounter() int {
	return f.lastUsedChangeOutputsCounter
}

func (f *noSendTransactionFixture) AllRemainedNoSendChange() []wdk.OutPoint {
	return stdslices.Collect(maps.Keys(f.allRemainedNoSendChange))
}

func (f *noSendTransactionFixture) CreateAction(args wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, *transaction.Transaction) {
	result, err := f.activeProvider.CreateAction(f.t.Context(), f.user.AuthID(), args)
	require.NoError(f.t, err)
	require.NotNil(f.t, result)

	tx, err := assembler.NewCreateActionTransactionAssembler(f.user.KeyDeriver(f.t), nil, result).Assemble()
	require.NoError(f.t, err)
	require.NotNil(f.t, tx)
	require.NoError(f.t, tx.Sign()) // <-- This is important

	f.lastCreateActionResult = result

	return result, tx
}

func (f *noSendTransactionFixture) WillSendSats(sats uint64) NoSendTransactionFixture {
	f.satsToSend = primitives.SatoshiValue(sats)
	return f
}

func (f *noSendTransactionFixture) ProcessAction(args wdk.ProcessActionArgs) *wdk.ProcessActionResult {
	result, err := f.activeProvider.ProcessAction(f.t.Context(), f.user.AuthID(), args)
	require.NoError(f.t, err)
	require.NotNil(f.t, result)

	return result
}

func (f *noSendTransactionFixture) CreateAndProcessNoSendAction(prevNoSendOutpoints []wdk.OutPoint) []wdk.OutPoint {
	createActionArgs := fixtures.DefaultValidCreateActionArgs(f.CreateActionNoSendArgsModifier(prevNoSendOutpoints, true))

	createActionResult, signedTx := f.CreateAction(createActionArgs)

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

	// update allRemainedNoSendChange
	f.lastUsedChangeOutputsCounter = 0
	for _, op := range noSendOutpoints {
		f.allRemainedNoSendChange[op] = struct{}{}
	}
	for _, input := range createActionResult.Inputs {
		outpoint := wdk.OutPoint{TxID: input.SourceTxID, Vout: input.SourceVout}

		if _, ok := f.allRemainedNoSendChange[outpoint]; ok {
			f.lastUsedChangeOutputsCounter++
			delete(f.allRemainedNoSendChange, outpoint)
		}
	}

	f.noSendTxsChain = append(f.noSendTxsChain, txID)

	return noSendOutpoints
}

func (f *noSendTransactionFixture) CreateAndProcessSendWithAction(sendWithHexStrings []primitives.HexString, opts ...func(*wdk.ValidCreateActionArgs)) (*wdk.ProcessActionResult, string) {
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

func (f *noSendTransactionFixture) CreateActionNoSendArgsModifier(prevNoSendOutpoints []wdk.OutPoint, isNoSend bool) func(args *wdk.ValidCreateActionArgs) {
	return func(args *wdk.ValidCreateActionArgs) {
		args.IsNewTx = true
		args.Outputs[0].Satoshis = f.satsToSend
		args.IsNoSend = isNoSend
		args.Options.NoSend = to.Ptr(primitives.BooleanDefaultFalse(isNoSend))
		args.Options.NoSendChange = prevNoSendOutpoints
	}
}

func (f *noSendTransactionFixture) CreateActionSendWithArgsModifier(sendWithHexStrings ...primitives.HexString) func(args *wdk.ValidCreateActionArgs) {
	return func(args *wdk.ValidCreateActionArgs) {
		args.IsSendWith = true
		args.Options.SendWith = sendWithHexStrings
	}
}
