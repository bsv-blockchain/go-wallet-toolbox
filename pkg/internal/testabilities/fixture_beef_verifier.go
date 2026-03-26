package testabilities

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/chaintracker"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type BeefVerifierFixture interface {
	WillReturnError(err error)
	WillReturnBool(value bool)
	DefaultBehavior()
}

type beefVerifierFixture struct {
	willReturnError error
	willReturnBool  *bool
}

func newBeefVerifierFixture() *beefVerifierFixture {
	return &beefVerifierFixture{}
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

func (b *beefVerifierFixture) Verifier(chaintracker chaintracker.ChainTracker) wdk.BeefVerifier {
	return &mockBeefVerifier{
		fixture:         b,
		defaultVerifier: storage.NewDefaultBeefVerifier(chaintracker),
	}
}

type mockBeefVerifier struct {
	fixture         *beefVerifierFixture
	defaultVerifier wdk.BeefVerifier
}

func (b *mockBeefVerifier) VerifyBeef(ctx context.Context, beef *transaction.Beef, allowTxidOnly bool) (bool, error) {
	if b.fixture.willReturnError != nil {
		return false, b.fixture.willReturnError
	}
	if b.fixture.willReturnBool != nil {
		return *b.fixture.willReturnBool, nil
	}
	return b.defaultVerifier.VerifyBeef(ctx, beef, allowTxidOnly)
}
