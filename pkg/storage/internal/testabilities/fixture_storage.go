package testabilities

import (
	"context"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities/tsgenerated"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/mocks"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/randomizer"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/database"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/server"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities/dbfixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type StorageFixture interface {
	Provider() ProviderFixture

	StartedRPCServerFor(provider wdk.WalletStorageWriter) (cleanup func())
	RPCClient() (*storage.WalletStorageWriterClient, func())

	MockProvider() *mocks.MockWalletStorageWriter

	Faucet(activeStorage *storage.Provider, user testusers.User) FaucetFixture

	ActionCreated(activeStorage *storage.Provider) (createActionResult *wdk.StorageCreateActionResult, signedTransaction *transaction.Transaction)
}

type FaucetFixture interface {
	TopUp(satoshis satoshi.Value) (txtestabilities.TransactionSpec, *models.UserUTXO)
}

type storageFixture struct {
	t          testing.TB
	require    *require.Assertions
	logger     *slog.Logger
	testServer *httptest.Server
	db         *database.Database
}

func (s *storageFixture) StartedRPCServerFor(provider wdk.WalletStorageWriter) (cleanup func()) {
	s.t.Helper()
	rpcServer := server.NewRPCHandler(s.logger, fixtures.StorageHandlerName, provider)

	mux := http.NewServeMux()
	rpcServer.Register(mux)

	s.testServer = httptest.NewServer(mux)
	return s.testServer.Close
}

func (s *storageFixture) RPCClient() (client *storage.WalletStorageWriterClient, cleanup func()) {
	s.t.Helper()
	client, cleanup, err := storage.NewClient(s.testServer.URL, storage.WithHttpClient(s.testServer.Client()))
	s.require.NoError(err)
	return client, cleanup
}

func (s *storageFixture) MockProvider() *mocks.MockWalletStorageWriter {
	s.t.Helper()
	ctrl := gomock.NewController(s.t)

	return mocks.NewMockWalletStorageWriter(ctrl)
}

func (s *storageFixture) Provider() ProviderFixture {
	s.t.Helper()
	return &providerFixture{
		t:       s.t,
		require: s.require,
		logger:  s.logger,
		db:      s.db,

		network:    defs.NetworkTestnet,
		commission: defs.Commission{},
		feeModel:   defs.DefaultFeeModel(),
		randomizer: randomizer.New(),
	}
}

func (s *storageFixture) Faucet(activeStorage *storage.Provider, user testusers.User) FaucetFixture {
	s.t.Helper()
	ctx := context.Background()

	_, err := activeStorage.FindOrInsertUser(ctx, user.PrivKey)
	s.require.NoError(err)

	basket, err := s.db.CreateRepositories().
		FindBasketByName(context.Background(), user.ID, wdk.BasketNameForChange)
	require.NoError(s.t, err)

	return &faucetFixture{
		t:        s.t,
		user:     user,
		db:       s.db,
		basketID: basket.BasketID,
	}
}

func (p *storageFixture) ActionCreated(activeStorage *storage.Provider) (createActionResult *wdk.StorageCreateActionResult, signedTransaction *transaction.Transaction) {
	ctx := context.Background()
	internalizeArgs := wdk.InternalizeActionArgs{
		Tx: tsgenerated.AtomicBeefToInternalize(p.t),
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
		Description:    "description",
	}

	// NOTE: Alice's identityKey has been used for tsgenerated.SignedTransaction - that's why you cannot use another user here
	user := testusers.Alice

	_, err := activeStorage.InternalizeAction(ctx, user.AuthID(), internalizeArgs)
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

	result, err := activeStorage.CreateAction(
		context.Background(),
		testusers.Alice.AuthID(),
		args,
	)

	require.NoError(p.t, err)

	return result, tsgenerated.SignedTransaction(p.t)
}

func Given(t testing.TB) StorageFixture {
	db, _ := dbfixtures.TestDatabase(t)
	return &storageFixture{
		t:       t,
		require: require.New(t),
		logger:  logging.NewTestLogger(t),
		db:      db,
	}
}
