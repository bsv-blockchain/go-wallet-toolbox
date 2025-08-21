package testabilities

import (
	"context"
	"time"
)

type MockStorage struct {
	SynchronizeTransactionStatusesCalled int
	SendWaitingTransactionsCalled        int
	SendWaitingLastMinTransactionAge     time.Duration
}

func (m *MockStorage) SynchronizeTransactionStatuses(_ context.Context) error {
	m.SynchronizeTransactionStatusesCalled++
	return nil
}

func (m *MockStorage) SendWaitingTransactions(_ context.Context, minTransactionAge time.Duration) error {
	m.SendWaitingTransactionsCalled++
	m.SendWaitingLastMinTransactionAge = minTransactionAge
	return nil
}
