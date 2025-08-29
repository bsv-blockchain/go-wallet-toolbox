package nosendtest

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
)

type NosendFixture interface {
	UserOwnsMultipleUTXOsToSpend(satoshis uint64)
	UserOwnsGivenUTXOsToSpend(satoshis ...uint64)
}

type nosendFixture struct {
	testing.TB
	storageFixture testabilities.StorageFixture
	user           testusers.User
	activeProvider *storage.Provider
}

func (f *nosendFixture) UserOwnsMultipleUTXOsToSpend(satoshis uint64) {
	f.storageFixture.
		Action(f.activeProvider).
		WithSender(f.user).
		WithRecipient(f.user).
		WithSatoshisToInternalize(satoshis).
		WithSatoshisToSend(1).
		Processed()
}

func (f *nosendFixture) UserOwnsGivenUTXOsToSpend(satoshis ...uint64) {
	faucet := f.storageFixture.Faucet(f.activeProvider, f.user)
	for _, s := range satoshis {
		faucet.TopUp(satoshi.MustFrom(s))
	}
}
