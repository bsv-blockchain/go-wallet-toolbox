package testabilities

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/transaction"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type ScriptsVerifierFixture interface {
	WillReturnError(err error)
	WillReturnBool(value bool)
	DefaultBehavior()
}

type scriptsVerifierFixture struct {
	willReturnError error
	willReturnBool  *bool
}

func newScriptsVerifierFixture() *scriptsVerifierFixture {
	return &scriptsVerifierFixture{}
}

func (s *scriptsVerifierFixture) WillReturnError(err error) {
	s.willReturnError = err
}

func (s *scriptsVerifierFixture) WillReturnBool(value bool) {
	s.willReturnBool = &value
}

func (s *scriptsVerifierFixture) DefaultBehavior() {
	s.willReturnError = nil
	s.willReturnBool = nil
}

func (s *scriptsVerifierFixture) Verifier() wdk.ScriptsVerifier {
	return &mockScriptsVerifier{
		fixture:         s,
		defaultVerifier: storage.NewDefaultScriptsVerifier(),
	}
}

type mockScriptsVerifier struct {
	fixture         *scriptsVerifierFixture
	defaultVerifier wdk.ScriptsVerifier
}

func (s *mockScriptsVerifier) VerifyScripts(ctx context.Context, tx *transaction.Transaction) (bool, error) {
	if s.fixture.willReturnError != nil {
		return false, s.fixture.willReturnError
	}
	if s.fixture.willReturnBool != nil {
		return *s.fixture.willReturnBool, nil
	}
	return s.defaultVerifier.VerifyScripts(ctx, tx)
}
