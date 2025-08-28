package testabilities

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/chaintracker"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
)

type BeefVerifierFixture interface {
	WillReturnError(err error)
	WillReturnBool(value bool)
	DefaultBehavior()
}

type beefVerifierFixture struct {
	defaultVerifier *storage.DefaultBeefVerifier
	willReturnError error
	willReturnBool  *bool
}

func newBeefVerifier() *beefVerifierFixture {
	return &beefVerifierFixture{
		defaultVerifier: &storage.DefaultBeefVerifier{},
	}
}

func (b *beefVerifierFixture) VerifyBeef(ctx context.Context, beef *transaction.Beef, chainTracker chaintracker.ChainTracker, allowTxidOnly bool) (bool, error) {
	if b.willReturnError != nil {
		return false, b.willReturnError
	}
	if b.willReturnBool != nil {
		return *b.willReturnBool, nil
	}
	return b.defaultVerifier.VerifyBeef(ctx, beef, chainTracker, allowTxidOnly)
}

func (b *beefVerifierFixture) WillReturnError(err error) {
	b.willReturnError = err
}

func (b *beefVerifierFixture) WillReturnBool(value bool) {
	b.willReturnBool = &value
}

func (b *beefVerifierFixture) DefaultBehavior() {
	b.willReturnError = nil
	b.willReturnBool = nil
}
