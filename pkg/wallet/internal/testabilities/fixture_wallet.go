package testabilities

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

type WalletFixture interface {
	AliceWalletWithStorage(storageType StorageType) (userWallet *wallet.Wallet, cleanup func())
	Wallet() WalletBuilder
}

type WalletBuilder interface {
	WithActiveStorage(storageType StorageType) WalletBuilder
	WithRemoteStorage() WalletBuilder
	WithSQLiteStorage() WalletBuilder
	ForUser(user testusers.User) (userWallet *wallet.Wallet, cleanup func())
}

type walletFixture struct {
	testing.TB
	usersSetups map[testusers.User]*userWalletSetup
}

func Given(t testing.TB) WalletFixture {
	return newGiven(t)
}

func newGiven(t testing.TB) *walletFixture {
	return &walletFixture{
		TB:          t,
		usersSetups: make(map[testusers.User]*userWalletSetup),
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

func (w *walletFixture) addUserWalletSetup(setup *userWalletSetup) {
	w.usersSetups[setup.user] = setup
}

type userWalletSetup struct {
	user        testusers.User
	wallet      *wallet.Wallet
	storage     wdk.WalletStorageProvider
	storageType StorageType
}
