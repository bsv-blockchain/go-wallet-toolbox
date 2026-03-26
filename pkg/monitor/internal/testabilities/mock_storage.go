package testabilities

import (
	"context"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type MockStorage struct {
	SynchronizeTransactionStatusesCalled int
	FailAbandonedCalled                  int

	SendWaitingTransactionsCalled    int
	SendWaitingLastMinTransactionAge time.Duration
	UnFailCalled                     int
	HandleReorgCalled                int
	ProcessNewTipCalled              int
}

func (m *MockStorage) SynchronizeTransactionStatuses(_ context.Context) ([]wdk.TxSynchronizedStatus, error) {
	m.SynchronizeTransactionStatusesCalled++
	return nil, nil
}

func (m *MockStorage) SendWaitingTransactions(_ context.Context, minTransactionAge time.Duration) (*wdk.ProcessActionResult, error) {
	m.SendWaitingTransactionsCalled++
	m.SendWaitingLastMinTransactionAge = minTransactionAge
	return nil, nil
}

func (m *MockStorage) AbortAbandoned(_ context.Context) error {
	m.FailAbandonedCalled++
	return nil
}

func (m *MockStorage) UnFail(_ context.Context) error {
	m.UnFailCalled++
	return nil
}

func (m *MockStorage) HandleReorg(_ context.Context, _ []string) error {
	m.HandleReorgCalled++
	return nil
}

func (m *MockStorage) ProcessNewTip(_ context.Context, _ uint32, _ string) ([]wdk.TxSynchronizedStatus, error) {
	m.ProcessNewTipCalled++
	return nil, nil
}
