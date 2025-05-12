package testabilities

import (
	"context"
	tsgenerated2 "github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities/tsgenerated"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
	"log/slog"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/database"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/require"
)

type ProviderFixture interface {
	WithNetwork(network defs.BSVNetwork) ProviderFixture
	WithCommission(commission defs.Commission) ProviderFixture
	WithFeeModel(feeModel defs.FeeModel) ProviderFixture
	WithRandomizer(randomizer wdk.Randomizer) ProviderFixture

	GORM() *storage.Provider
	GORMWithCleanDatabase() *storage.Provider

	ActionCreated(user testusers.User) (createActionResult *wdk.StorageCreateActionResult, signedTransaction *transaction.Transaction)
}

type providerFixture struct {
	network    defs.BSVNetwork
	commission defs.Commission
	feeModel   defs.FeeModel
	randomizer wdk.Randomizer

	t             testing.TB
	require       *require.Assertions
	logger        *slog.Logger
	db            *database.Database
	activeStorage *storage.Provider
}

func (p *providerFixture) WithNetwork(network defs.BSVNetwork) ProviderFixture {
	p.network = network
	return p
}

func (p *providerFixture) WithCommission(commission defs.Commission) ProviderFixture {
	p.commission = commission
	return p
}

func (p *providerFixture) WithFeeModel(feeModel defs.FeeModel) ProviderFixture {
	p.feeModel = feeModel
	return p
}

func (p *providerFixture) WithRandomizer(randomizer wdk.Randomizer) ProviderFixture {
	p.randomizer = randomizer
	return p
}

func (p *providerFixture) GORM() *storage.Provider {
	p.t.Helper()
	provider := p.GORMWithCleanDatabase()

	p.seedUsers(provider)

	return provider
}

func (p *providerFixture) GORMWithCleanDatabase() *storage.Provider {
	p.t.Helper()

	storageIdentityKey, err := wdk.IdentityKey(fixtures.StorageServerPrivKey)
	p.require.NoError(err)

	activeStorage, err := storage.NewGORMProvider(
		p.logger,
		storage.GORMProviderConfig{
			Chain:      p.network,
			FeeModel:   p.feeModel,
			Commission: p.commission,
		},
		storage.WithGORM(p.db.DB),
		storage.WithRandomizer(p.randomizer),
	)
	p.require.NoError(err)

	_, err = activeStorage.Migrate(context.Background(), fixtures.StorageName, storageIdentityKey)
	p.require.NoError(err)

	p.activeStorage = activeStorage

	return activeStorage
}

func (p *providerFixture) ActionCreated(user testusers.User) (createActionResult *wdk.StorageCreateActionResult, signedTransaction *transaction.Transaction) {
	ctx := context.Background()
	internalizeArgs := wdk.InternalizeActionArgs{
		Tx: tsgenerated2.AtomicBeefToInternalize(p.t),
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
	}

	_, err := p.activeStorage.InternalizeAction(ctx, user.AuthID(), internalizeArgs)
	require.NoError(p.t, err)

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

	result, err := p.activeStorage.CreateAction(
		context.Background(),
		testusers.Alice.AuthID(),
		args,
	)

	require.NoError(p.t, err)

	return result, tsgenerated2.SignedTransaction(p.t)
}

func (p *providerFixture) seedUsers(provider *storage.Provider) {
	for _, user := range testusers.All() {
		res, err := provider.FindOrInsertUser(context.Background(), user.PrivKey)
		p.require.NoError(err)

		user.ID = res.User.UserID
	}
}
