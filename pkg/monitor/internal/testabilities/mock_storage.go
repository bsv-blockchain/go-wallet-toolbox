package testabilities

import (
	"context"
	"time"
)

type MockStorage struct {
	SynchronizeTransactionStatusesCalled int
	SendWaitingTransactionsCalled        int
	SendWaitingLastAgedLimit             time.Duration
}

func (m *MockStorage) SynchronizeTransactionStatuses(_ context.Context) error {
	m.SynchronizeTransactionStatusesCalled++
	return nil
}

func (m *MockStorage) SendWaitingTransactions(_ context.Context, agedLimit time.Duration) error {
	m.SendWaitingTransactionsCalled++
	m.SendWaitingLastAgedLimit = agedLimit
	return nil
}
