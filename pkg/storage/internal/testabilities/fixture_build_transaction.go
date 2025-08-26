package testabilities

import (
	stdslices "slices"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type NotDelayedResultsAsserter []wdk.ReviewActionResult

func (n NotDelayedResultsAsserter) ContainsTxsWithStatus(t *testing.T, status wdk.ReviewActionResultStatus, txIDs ...string) {
	for _, txID := range txIDs {
		idx := stdslices.IndexFunc(n, func(i wdk.ReviewActionResult) bool {
			return i.TxID == primitives.TXIDHexString(txID)
		})

		assert.GreaterOrEqual(t, idx, 0, "Expected to find txID %s in not delayed results", txID)
		assert.Equal(t, status, n[idx].Status, "Expected status for txID %s to be %s, got %s", txID, status, n[idx].Status)
	}
}

type SendWithResultsAsserter []wdk.SendWithResult

func (r SendWithResultsAsserter) ContainsTxsWithStatus(t *testing.T, status wdk.SendWithResultStatus, txIDs ...string) {
	for _, txID := range txIDs {
		idx := stdslices.IndexFunc(r, func(i wdk.SendWithResult) bool {
			return i.TxID == primitives.TXIDHexString(txID)
		})

		assert.GreaterOrEqual(t, idx, 0, "Expected to find txID %s in send with results", txID)
		assert.Equal(t, status, r[idx].Status, "Expected status for txID %s to be %s, got %s", txID, status, r[idx].Status)
	}
}

type BuildNoSendTransactionFixture struct {
	t              *testing.T
	cleanup        func()
	activeStorage  *storage.Provider
	storageFixture StorageFixture
	deriver        *wallet.KeyDeriver
	userAuthID     wdk.AuthID
}

func (b *BuildNoSendTransactionFixture) UserAuthID() wdk.AuthID { return b.userAuthID }

func NewBuildNoSendTransactionFixture(t *testing.T, satoshish uint64) *BuildNoSendTransactionFixture {
	storageFixture, cleanup := Given(t)
	activeStorage := storageFixture.Provider().GORM()

	storageFixture.
		Action(activeStorage).
		WithSatoshisToInternalize(satoshish).
		WithSatoshisToSend(1).
		Processed()

	return &BuildNoSendTransactionFixture{
		t:              t,
		deriver:        testusers.Alice.KeyDeriver(t),
		userAuthID:     testusers.Alice.AuthID(),
		activeStorage:  activeStorage,
		storageFixture: storageFixture,
		cleanup:        cleanup,
	}
}

func (f *BuildNoSendTransactionFixture) ActiveStorage() *storage.Provider { return f.activeStorage }

func (f *BuildNoSendTransactionFixture) Cleanup() { f.cleanup() }

func (f *BuildNoSendTransactionFixture) CreateAction(args wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, *transaction.Transaction) {
	result, err := f.activeStorage.CreateAction(f.t.Context(), f.userAuthID, args)
	require.NoError(f.t, err)
	require.NotNil(f.t, result)

	tx, err := assembler.NewCreateActionTransactionAssembler(f.deriver, nil, result).Assemble()
	require.NoError(f.t, err)
	require.NotNil(f.t, tx)
	require.NoError(f.t, tx.Sign())

	return result, tx
}

func (f *BuildNoSendTransactionFixture) ProcessAction(userAuthID wdk.AuthID, args wdk.ProcessActionArgs) *wdk.ProcessActionResult {
	result, err := f.activeStorage.ProcessAction(f.t.Context(), userAuthID, args)
	require.NoError(f.t, err)
	require.NotNil(f.t, result)

	return result
}

type BuildNoSendTransactionResult struct {
	// Step 1:
	FirstCreateActionResult *wdk.StorageCreateActionResult
	FirstTxID               string
	FirstCreateActionArgs   wdk.ValidCreateActionArgs

	// Step 2:
	SecondCreateActionResult *wdk.StorageCreateActionResult
	SecondTxID               string
	SecondCreateActionArgs   wdk.ValidCreateActionArgs
}

func (f *BuildNoSendTransactionFixture) BuildNoSendTransaction() *BuildNoSendTransactionResult {
	// Step 1. Process the action with the IsNoSend flag set to true, leaving the noSendChange outpoints empty. Inputs will still be allocated normally from spendable outputs (change basket).
	firstActionArgs := fixtures.DefaultValidCreateActionArgs(func(args *wdk.ValidCreateActionArgs) {
		args.Outputs[0].Satoshis = 1
		args.IsNoSend = true
		args.Options.NoSend = to.Ptr(primitives.BooleanDefaultFalse(true))
	})

	firstCreateActionResult, firstTx := f.CreateAction(firstActionArgs)
	require.NotEmpty(f.t, firstCreateActionResult.NoSendChangeOutputVouts)

	firstTxID := firstTx.TxID().String()

	f.ProcessAction(testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx:   true,
		IsNoSend:  true,
		Reference: to.Ptr(firstCreateActionResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(firstTxID)),
		RawTx:     firstTx.Bytes(),
	})

	// Step 2. Create an Action with NoSendChange using the result of the previous Create Action.
	secondCreateArgs := fixtures.DefaultValidCreateActionArgs(func(args *wdk.ValidCreateActionArgs) {
		args.Outputs[0].Satoshis = 1
		args.IsNoSend = true
		args.Options.NoSend = to.Ptr(primitives.BooleanDefaultFalse(true))
		args.Options.NoSendChange = slices.Map(firstCreateActionResult.NoSendChangeOutputVouts, func(vout int) wdk.OutPoint {
			return wdk.OutPoint{TxID: firstTxID, Vout: uint32(vout)}
		})
	})

	secondCreateActionResult, secondTx := f.CreateAction(secondCreateArgs)
	secondTxID := secondTx.TxID().String()

	f.ProcessAction(testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx:   true,
		IsNoSend:  true,
		Reference: to.Ptr(secondCreateActionResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(secondTxID)),
		RawTx:     secondTx.Bytes(),
	})

	return &BuildNoSendTransactionResult{
		FirstCreateActionResult:  firstCreateActionResult,
		FirstTxID:                firstTxID,
		FirstCreateActionArgs:    firstActionArgs,
		SecondCreateActionResult: secondCreateActionResult,
		SecondTxID:               secondTxID,
		SecondCreateActionArgs:   secondCreateArgs,
	}
}
