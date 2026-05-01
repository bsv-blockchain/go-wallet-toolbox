package testabilities

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type MockStorage struct {
	SynchronizeTransactionStatusesCalled atomic.Int64
	FailAbandonedCalled                  atomic.Int64

	SendWaitingTransactionsCalled    atomic.Int64
	SendWaitingLastMinTransactionAge time.Duration
	UnFailCalled                     atomic.Int64
	HandleReorgCalled                atomic.Int64
	ProcessNewTipCalled              atomic.Int64
}

func (m *MockStorage) SynchronizeTransactionStatuses(_ context.Context) ([]wdk.TxSynchronizedStatus, error) {
	m.SynchronizeTransactionStatusesCalled.Add(1)
	return nil, nil
}

func (m *MockStorage) SendWaitingTransactions(_ context.Context, minTransactionAge time.Duration) (*wdk.ProcessActionResult, error) {
	m.SendWaitingTransactionsCalled.Add(1)
	m.SendWaitingLastMinTransactionAge = minTransactionAge
	return nil, nil
}

func (m *MockStorage) AbortAbandoned(_ context.Context) error {
	m.FailAbandonedCalled.Add(1)
	return nil
}

func (m *MockStorage) UnFail(_ context.Context) error {
	m.UnFailCalled.Add(1)
	return nil
}

func (m *MockStorage) HandleReorg(_ context.Context, _ []string) error {
	m.HandleReorgCalled.Add(1)
	return nil
}

func (m *MockStorage) ProcessNewTip(_ context.Context, _ uint32, _ string) ([]wdk.TxSynchronizedStatus, error) {
	m.ProcessNewTipCalled.Add(1)
	return nil, nil
}
