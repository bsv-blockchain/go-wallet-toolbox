package testabilities

import (
	"context"
	"testing"

	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/walletargs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type CreateActionInputBuilder = walletargs.CreateActionInputBuilder

// mockFacilitator implements lookup.Facilitator interface for testing
type mockFacilitator struct {
	answer *lookup.LookupAnswer
	err    error
}

func (m *mockFacilitator) Lookup(_ context.Context, _ string, _ *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	return m.answer, m.err
}

type WalletFixture interface {
	AliceWalletWithStorage(storageType StorageType) *wallet.Wallet
	BobWalletWithStorage(storageType StorageType) (userWallet *wallet.Wallet)
	Wallet() WalletBuilder
	Faucet(userWallet *wallet.Wallet) FaucetFixture
	InputForUser(user testusers.User) CreateActionInputBuilder
	Services() ServicesFixture
	BeefVerifier() testabilities.BeefVerifierFixture
	ScriptsVerifier() testabilities.ScriptsVerifierFixture
	CertifierServer() CertifierServerBuilder
	MockLookupResolver(answer *lookup.LookupAnswer, err error) *lookup.LookupResolver
}

type walletFixture struct {
	testing.TB

	storageFixture testabilities.StorageFixture
	usersSetups    map[testusers.User]*userWalletSetup
	usersFaucets   map[string]*faucetFixture
	cleanupFuncs   []func()
}

func Given(t testing.TB) (given WalletFixture, cleanup func()) {
	return newGiven(t)
}

func (w *walletFixture) MockLookupResolver(answer *lookup.LookupAnswer, err error) *lookup.LookupResolver {
	return lookup.NewLookupResolver(&lookup.LookupResolver{
		Facilitator: &mockFacilitator{answer: answer, err: err},
		// Use HostOverrides to bypass SLAP tracker lookup
		HostOverrides: map[string][]string{
			"ls_identity": {"http://mock-host"},
		},
	})
}

func newGiven(t testing.TB) (given *walletFixture, cleanup func()) {
	storageFixture, storageCleanup := testabilities.Given(t)

	w := &walletFixture{
		TB:             t,
		usersSetups:    make(map[testusers.User]*userWalletSetup),
		usersFaucets:   make(map[string]*faucetFixture),
		cleanupFuncs:   []func(){storageCleanup},
		storageFixture: storageFixture,
	}

	cleanup = func() {
		for cleanupFunc := range seq.FromSliceReversed(w.cleanupFuncs) {
			cleanupFunc()
		}
	}

	return w, cleanup
}

func (w *walletFixture) CertifierServer() CertifierServerBuilder {
	return &certifierServerBuilder{
		TB: w.TB,
	}
}

func (w *walletFixture) AliceWalletWithStorage(storageType StorageType) *wallet.Wallet {
	return w.Wallet().WithActiveStorage(storageType).WithServices().ForUser(testusers.Alice)
}

func (w *walletFixture) BobWalletWithStorage(storageType StorageType) (userWallet *wallet.Wallet) {
	return w.Wallet().WithActiveStorage(storageType).WithServices().ForUser(testusers.Bob)
}

func (w *walletFixture) Wallet() WalletBuilder {
	return &walletBuilder{
		TB:            w.TB,
		givenStorage:  w.storageFixture,
		walletFixture: w,
	}
}

func (w *walletFixture) BeefVerifier() testabilities.BeefVerifierFixture {
	return w.storageFixture.Provider().BeefVerifier()
}

func (w *walletFixture) ScriptsVerifier() testabilities.ScriptsVerifierFixture {
	return w.storageFixture.Provider().ScriptsVerifier()
}

func (w *walletFixture) Faucet(userWallet *wallet.Wallet) FaucetFixture {
	publicKey, err := userWallet.GetPublicKey(w.Context(), sdk.GetPublicKeyArgs{IdentityKey: true}, "")
	require.NoError(w, err, "Failed to retrieve identity key from wallet to top up")

	identityKey := publicKey.PublicKey.ToDERHex()

	faucet, ok := w.usersFaucets[identityKey]
	if !ok {
		faucet = &faucetFixture{
			TB:         w.TB,
			userWallet: userWallet,
			index:      0,
		}
		w.usersFaucets[identityKey] = faucet
	}

	return faucet
}

func (w *walletFixture) InputForUser(user testusers.User) CreateActionInputBuilder {
	return walletargs.NewCreateActionInputBuilder(w.TB, user)
}

func (w *walletFixture) Services() ServicesFixture {
	return &servicesFixture{
		ServicesFixture: w.storageFixture.Provider(),
	}
}

func (w *walletFixture) addUserWalletSetup(setup *userWalletSetup) {
	w.usersSetups[setup.user] = setup
	if setup.cleanupFunc != nil {
		w.cleanupFuncs = append(w.cleanupFuncs, setup.cleanupFunc)
	}
}

type userWalletSetup struct {
	user        testusers.User
	wallet      *wallet.Wallet
	storage     wdk.WalletStorageProvider
	storageType StorageType
	cleanupFunc func()
}
