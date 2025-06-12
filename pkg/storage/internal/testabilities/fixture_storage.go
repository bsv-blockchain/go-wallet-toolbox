package testabilities

import (
	"context"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities/tsgenerated"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

type StorageFixture interface {
	testabilities.StorageFixture
	StorageManagerForUser(user testusers.User, activeStorage wdk.WalletStorageProvider) *storage.WalletStorageManager
	ActionCreatedAndSigned(activeStorage *storage.Provider) (createActionResult *wdk.StorageCreateActionResult, signedTransaction *transaction.Transaction)
}

type storageFixture struct {
	t testing.TB
	testabilities.StorageFixture
}

func (s *storageFixture) StorageManagerForUser(user testusers.User, activeStorage wdk.WalletStorageProvider) *storage.WalletStorageManager {
	return storage.NewWalletStorageManager(user.IdentityKey(s.t), activeStorage)
}

func (s *storageFixture) ActionCreatedAndSigned(activeStorage *storage.Provider) (createActionResult *wdk.StorageCreateActionResult, signedTransaction *transaction.Transaction) {
	ctx := context.Background()
	internalizeArgs := wdk.InternalizeActionArgs{
		Tx: tsgenerated.AtomicBeefToInternalize(s.t),
		Outputs: []*wdk.InternalizeOutput{
			{
				OutputIndex: 0,
				Protocol:    wdk.WalletPaymentProtocol,
				PaymentRemittance: &wdk.WalletPayment{
					DerivationPrefix:  fixtures.DerivationPrefix,
					DerivationSuffix:  fixtures.DerivationSuffix,
					SenderIdentityKey: fixtures.AnyoneIdentityKey,
				},
			},
		},
		Description: "description",
	}

	// NOTE: Alice's identityKey has been used for tsgenerated.SignedTransaction - that's why you cannot use another user here
	user := testusers.Alice

	_, err := activeStorage.InternalizeAction(ctx, user.AuthID(), internalizeArgs)
	require.NoError(s.t, err)

	args := wdk.ValidCreateActionArgs{
		Description: "outputBRC29",
		Inputs:      []wdk.ValidCreateActionInput{},
		Outputs: []wdk.ValidCreateActionOutput{
			{
				LockingScript:      "76a9144b0d6cbef5a813d2d12dcec1de2584b250dc96a388ac",
				Satoshis:           1000,
				OutputDescription:  "outputBRC29",
				CustomInstructions: to.Ptr(`{"derivationPrefix":"Pr==","derivationSuffix":"Su==","type":"BRC29"}`),
			},
		},
		LockTime: 0,
		Version:  1,
		Labels:   []primitives.StringUnder300{"outputbrc29"},
		Options: wdk.ValidCreateActionOptions{
			AcceptDelayedBroadcast: to.Ptr[primitives.BooleanDefaultTrue](false),
			SendWith:               []primitives.TXIDHexString{},
			SignAndProcess:         to.Ptr(primitives.BooleanDefaultTrue(true)),
			KnownTxids:             []primitives.TXIDHexString{},
			NoSendChange:           []wdk.OutPoint{},
			RandomizeOutputs:       false,
		},
		IsSendWith:                   false,
		IsDelayed:                    false,
		IsNoSend:                     false,
		IsNewTx:                      true,
		IsRemixChange:                false,
		IsSignAction:                 false,
		IncludeAllSourceTransactions: true,
	}

	result, err := activeStorage.CreateAction(
		context.Background(),
		testusers.Alice.AuthID(),
		args,
	)

	require.NoError(s.t, err)

	return result, tsgenerated.SignedTransaction(s.t)
}

func Given(t testing.TB) (given StorageFixture, cleanup func()) {
	storageFxt, cleanupFunc := testabilities.Given(t)

	return &storageFixture{
		t:              t,
		StorageFixture: storageFxt,
	}, cleanupFunc
}

func GivenCustomStorage(t testing.TB, identityKey string, name string) (given StorageFixture, cleanup func()) {
	storageFxt, cleanupFunc := testabilities.GivenCustomStorage(t, identityKey, name)

	return &storageFixture{
		t:              t,
		StorageFixture: storageFxt,
	}, cleanupFunc
}
