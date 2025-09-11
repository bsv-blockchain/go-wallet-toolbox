package testabilities

import (
	"context"
	"time"
)

type MockStorage struct {
	SynchronizeTransactionStatusesCalled int
	FailAbandonedCalled                  int

	SendWaitingTransactionsCalled    int
	SendWaitingLastMinTransactionAge time.Duration
	CheckFailedTransactionsCalled    int
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

func (m *MockStorage) AbortAbandoned(_ context.Context) error {
	m.FailAbandonedCalled++
	return nil
}

func (m *MockStorage) CheckFailedTransactions(_ context.Context) error {
	m.CheckFailedTransactionsCalled++
	return nil
}
