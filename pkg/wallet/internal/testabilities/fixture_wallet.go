package testabilities

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"
)

type WalletFixture interface {
	AliceWalletWithStorage(storageType StorageType) (userWallet *wallet.Wallet, cleanup func())
	Wallet() WalletBuilder
	Faucet(userWallet *wallet.Wallet) FaucetFixture
	InputForUser(user testusers.User) CreateActionInputBuilder
}

type CreateActionInputBuilder interface {
	WithDescription(description string) CreateActionInputBuilder
	WithSatoshis(satoshis int) CreateActionInputBuilder
	CreateActionInput() sdk.CreateActionInput
	InputBEEFBytes() []byte
}

type WalletBuilder interface {
	WithActiveStorage(storageType StorageType) WalletBuilder
	WithRemoteStorage() WalletBuilder
	WithSQLiteStorage() WalletBuilder
	ForUser(user testusers.User) (userWallet *wallet.Wallet, cleanup func())
}

type walletFixture struct {
	testing.TB
	usersSetups  map[testusers.User]*userWalletSetup
	usersFaucets map[string]*faucetFixture
}

func Given(t testing.TB) WalletFixture {
	return newGiven(t)
}

func newGiven(t testing.TB) *walletFixture {
	return &walletFixture{
		TB:           t,
		usersSetups:  make(map[testusers.User]*userWalletSetup),
		usersFaucets: make(map[string]*faucetFixture),
	}
}

func (w *walletFixture) AliceWalletWithStorage(storageType StorageType) (userWallet *wallet.Wallet, cleanup func()) {
	return w.Wallet().WithActiveStorage(storageType).ForUser(testusers.Alice)
}

func (w *walletFixture) Wallet() WalletBuilder {
	return &walletBuilder{
		TB:            w.TB,
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
	return &createActionInputBuilder{
		TB:          w.TB,
		user:        user,
		description: "self provided input from tests",
		satoshis:    1,
	}
}

func (w *walletFixture) addUserWalletSetup(setup *userWalletSetup) {
	w.usersSetups[setup.user] = setup
}

type userWalletSetup struct {
	user        testusers.User
	wallet      *wallet.Wallet
	storage     wdk.WalletStorageProvider
	storageType StorageType
}
