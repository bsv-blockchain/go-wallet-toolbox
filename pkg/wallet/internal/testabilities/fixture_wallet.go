package testabilities

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/walletargs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/stretchr/testify/require"
)

type CreateActionInputBuilder = walletargs.CreateActionInputBuilder

type WalletFixture interface {
	AliceWalletWithStorage(storageType StorageType) *wallet.Wallet
	BobWalletWithStorage(storageType StorageType) (userWallet *wallet.Wallet)
	Wallet() WalletBuilder
	Faucet(userWallet *wallet.Wallet) FaucetFixture
	InputForUser(user testusers.User) CreateActionInputBuilder
	Services() ServicesFixture
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

func (w *walletFixture) AliceWalletWithStorage(storageType StorageType) *wallet.Wallet {
	return w.Wallet().WithActiveStorage(storageType).ForUser(testusers.Alice)
}

func (w *walletFixture) BobWalletWithStorage(storageType StorageType) (userWallet *wallet.Wallet) {
	return w.Wallet().WithActiveStorage(storageType).ForUser(testusers.Bob)
}

func (w *walletFixture) Wallet() WalletBuilder {
	return &walletBuilder{
		TB:            w.TB,
		givenStorage:  w.storageFixture,
		walletFixture: w,
	}
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
