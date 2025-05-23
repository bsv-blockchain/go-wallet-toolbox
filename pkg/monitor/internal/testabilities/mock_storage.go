package testabilities

import "context"

type MockStorage struct {
	SynchronizeTransactionStatusesCalled int
}

func (m *MockStorage) SynchronizeTransactionStatuses(_ context.Context) error {
	m.SynchronizeTransactionStatusesCalled++
	return nil
}
