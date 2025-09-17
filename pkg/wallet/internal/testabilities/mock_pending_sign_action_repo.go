package testabilities

import (
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/pending"
)

type MockPendingSignActionRepo struct {
	base        *pending.SignActionLocalRepository
	ErrOnSet    error
	ErrOnGet    error
	ErrOnDelete error
}

func NewMockPendingSignActionCache() *MockPendingSignActionRepo {
	return &MockPendingSignActionRepo{
		base: pending.NewSignActionLocalRepository(slog.Default(), -1),
	}
}

func (m *MockPendingSignActionRepo) Save(reference string, action *pending.SignAction) error {
	if m.ErrOnSet != nil {
		return m.ErrOnSet
	}

	err := m.base.Save(reference, action)
	if err != nil {
		return fmt.Errorf("mock base Save error: %w", err)
	}

	return nil
}

func (m *MockPendingSignActionRepo) Get(reference string) (*pending.SignAction, error) {
	if m.ErrOnGet != nil {
		return nil, m.ErrOnGet
	}

	return m.base.Get(reference)
}

func (m *MockPendingSignActionRepo) Delete(reference string) error {
	if m.ErrOnDelete != nil {
		return m.ErrOnDelete
	}

	return m.base.Delete(reference)
}
