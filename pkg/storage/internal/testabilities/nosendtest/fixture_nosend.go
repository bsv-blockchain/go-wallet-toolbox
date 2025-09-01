package nosendtest

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
)

type NoSendFixture interface {
	testabilities.StorageFixture

	ActiveProvider() *storage.Provider
	UserOwnsMultipleUTXOsToSpend(satoshis uint64)
	UserOwnsGivenUTXOsToSpend(satoshis ...uint64)
}

type noSendFixture struct {
	testing.TB
	testabilities.StorageFixture

	user           testusers.User
	activeProvider *storage.Provider
}

func (f *noSendFixture) UserOwnsMultipleUTXOsToSpend(satoshis uint64) {
	f.Action(f.activeProvider).
		WithSender(f.user).
		WithRecipient(f.user).
		WithSatoshisToInternalize(satoshis).
		WithSatoshisToSend(1).
		Processed()
}

func (f *noSendFixture) UserOwnsGivenUTXOsToSpend(satoshis ...uint64) {
	faucet := f.Faucet(f.activeProvider, f.user)
	for _, s := range satoshis {
		faucet.TopUp(satoshi.MustFrom(s))
	}
}

func (f *noSendFixture) ActiveProvider() *storage.Provider {
	return f.activeProvider
}
