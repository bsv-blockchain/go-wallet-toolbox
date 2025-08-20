package testabilities

import (
	"context"
	"time"
)

type MockStorage struct {
	SynchronizeTransactionStatusesCalled int
	SendWaitingTransactionsCalled        int
}

func (m *MockStorage) SynchronizeTransactionStatuses(_ context.Context) error {
	m.SynchronizeTransactionStatusesCalled++
	return nil
}

func (m *MockStorage) SendWaitingTransactions(ctx context.Context, agedLimit time.Duration) error {
	m.SendWaitingTransactionsCalled++
	return nil
}
