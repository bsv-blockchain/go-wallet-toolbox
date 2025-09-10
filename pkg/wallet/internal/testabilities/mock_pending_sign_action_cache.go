package testabilities

import (
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/pending"
)

type MockPendingSignActionCache struct {
	base        *pending.SignActionLocalRepository
	ErrOnSet    error
	ErrOnGet    error
	ErrOnDelete error
}

func NewMockPendingSignActionCache() *MockPendingSignActionCache {
	return &MockPendingSignActionCache{
		base: pending.NewSignActionLocalRepository(slog.Default(), -1),
	}
}

func (m *MockPendingSignActionCache) Set(reference string, action *pending.SignAction) error {
	if m.ErrOnSet != nil {
		return m.ErrOnSet
	}

	return m.base.Set(reference, action)
}

func (m *MockPendingSignActionCache) Get(reference string) (*pending.SignAction, error) {
	if m.ErrOnGet != nil {
		return nil, m.ErrOnGet
	}

	return m.base.Get(reference)
}

func (m *MockPendingSignActionCache) Delete(reference string) error {
	if m.ErrOnDelete != nil {
		return m.ErrOnDelete
	}

	return m.base.Delete(reference)
}
