package nosendtest

import (
	"maps"
	stdslices "slices"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

type NoSendAct interface {
	WillSendSats(sats uint64) NoSendAct
	NoSendTxsHexStrings() []primitives.HexString
	NoSendTxs() []string
	AllRemainedNoSendChange() []wdk.OutPoint
	LastCreateActionResult() *wdk.StorageCreateActionResult

	CreateAction(args wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, *transaction.Transaction)
	ProcessAction(args wdk.ProcessActionArgs) *wdk.ProcessActionResult
	CreateAndProcessNoSendAction(prevNoSendOutpoints []wdk.OutPoint) (createdNoSendChange, allocatedNoSendChangeAsInputs []wdk.OutPoint)
	CreateActionNoSendArgsModifier(prevNoSendOutpoints []wdk.OutPoint, isNoSend bool) func(args *wdk.ValidCreateActionArgs)
	CreateActionSendWithArgsModifier(sendWithHexStrings ...primitives.HexString) func(args *wdk.ValidCreateActionArgs)
	CreateAndProcessSendWithAction(sendWithHexStrings []primitives.HexString, opts ...func(*wdk.ValidCreateActionArgs)) (*wdk.ProcessActionResult, string)
}

type noSendAct struct {
	testing.TB

	user                    testusers.User
	activeProvider          *storage.Provider
	lastCreateActionResult  *wdk.StorageCreateActionResult
	satsToSend              primitives.SatoshiValue
	allRemainedNoSendChange map[wdk.OutPoint]struct{}
	noSendTxsChain          []string
}

func (f *noSendAct) WillSendSats(sats uint64) NoSendAct {
	f.satsToSend = primitives.SatoshiValue(sats)
	return f
}

func (f *noSendAct) NoSendTxsHexStrings() []primitives.HexString {
	return slices.Map(f.noSendTxsChain, func(s string) primitives.HexString { return primitives.HexString(s) })
}

func (f *noSendAct) NoSendTxs() []string {
	return f.noSendTxsChain
}

func (f *noSendAct) AllRemainedNoSendChange() []wdk.OutPoint {
	return stdslices.Collect(maps.Keys(f.allRemainedNoSendChange))
}

func (f *noSendAct) LastCreateActionResult() *wdk.StorageCreateActionResult {
	return f.lastCreateActionResult
}

func (f *noSendAct) CreateAction(args wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, *transaction.Transaction) {
	result, err := f.activeProvider.CreateAction(f.Context(), f.user.AuthID(), args)
	require.NoError(f, err)
	require.NotNil(f, result)

	tx, err := assembler.NewCreateActionTransactionAssembler(f.user.KeyDeriver(f), nil, result).Assemble()
	require.NoError(f, err)
	require.NotNil(f, tx)
	require.NoError(f, tx.Sign()) // <-- This is important

	f.lastCreateActionResult = result

	return result, tx.Transaction
}

func (f *noSendAct) ProcessAction(args wdk.ProcessActionArgs) *wdk.ProcessActionResult {
	result, err := f.activeProvider.ProcessAction(f.Context(), f.user.AuthID(), args)
	require.NoError(f, err)
	require.NotNil(f, result)

	return result
}

func (f *noSendAct) CreateAndProcessNoSendAction(prevNoSendOutpoints []wdk.OutPoint) (createdNoSendChange, allocatedNoSendChangeAsInputs []wdk.OutPoint) {
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

	createdNoSendChange = slices.Map(createActionResult.NoSendChangeOutputVouts, func(vout int) wdk.OutPoint {
		return wdk.OutPoint{
			TxID: txID,
			Vout: uint32(vout), //nolint:gosec // test fixture, vout is always small
		}
	})

	allocatedNoSendChangeAsInputs = f.updateAllRemainedNoSendChange(createActionResult, createdNoSendChange)
	f.noSendTxsChain = append(f.noSendTxsChain, txID)

	return createdNoSendChange, allocatedNoSendChangeAsInputs
}

func (f *noSendAct) updateAllRemainedNoSendChange(createActionResult *wdk.StorageCreateActionResult, createdNoSendChange []wdk.OutPoint) []wdk.OutPoint {
	var allocatedNoSendChangeAsInputs []wdk.OutPoint
	for _, op := range createdNoSendChange {
		f.allRemainedNoSendChange[op] = struct{}{}
	}
	for _, input := range createActionResult.Inputs {
		outpoint := wdk.OutPoint{TxID: input.SourceTxID, Vout: input.SourceVout}

		if _, ok := f.allRemainedNoSendChange[outpoint]; ok {
			allocatedNoSendChangeAsInputs = append(allocatedNoSendChangeAsInputs, outpoint)
			delete(f.allRemainedNoSendChange, outpoint)
		}
	}

	return allocatedNoSendChangeAsInputs
}

func (f *noSendAct) CreateAndProcessSendWithAction(sendWithHexStrings []primitives.HexString, opts ...func(*wdk.ValidCreateActionArgs)) (*wdk.ProcessActionResult, string) {
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

func (f *noSendAct) CreateActionNoSendArgsModifier(prevNoSendOutpoints []wdk.OutPoint, isNoSend bool) func(args *wdk.ValidCreateActionArgs) {
	return func(args *wdk.ValidCreateActionArgs) {
		args.IsNewTx = true
		args.Outputs[0].Satoshis = f.satsToSend
		args.IsNoSend = isNoSend
		args.Options.NoSend = to.Ptr(primitives.BooleanDefaultFalse(isNoSend))
		args.Options.NoSendChange = prevNoSendOutpoints
	}
}

func (f *noSendAct) CreateActionSendWithArgsModifier(sendWithHexStrings ...primitives.HexString) func(args *wdk.ValidCreateActionArgs) {
	return func(args *wdk.ValidCreateActionArgs) {
		args.IsSendWith = true
		args.Options.SendWith = sendWithHexStrings
	}
}
