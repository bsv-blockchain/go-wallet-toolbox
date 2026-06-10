package testabilities

import (
	"context"
	"sync"
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

	ProcessExternalTxStatusUpdateCalled atomic.Int64
	GetKeyValueCalled                   atomic.Int64
	SetKeyValueCalled                   atomic.Int64

	mu             sync.Mutex
	keyValues      map[string][]byte
	externalEvents []wdk.BroadcastStatusEvent
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

func (m *MockStorage) ProcessExternalTxStatusUpdate(_ context.Context, ev wdk.BroadcastStatusEvent) ([]wdk.TxSynchronizedStatus, error) {
	m.ProcessExternalTxStatusUpdateCalled.Add(1)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.externalEvents = append(m.externalEvents, ev)

	return nil, nil
}

// ExternalEvents returns the external status events received so far (in order).
func (m *MockStorage) ExternalEvents() []wdk.BroadcastStatusEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	events := make([]wdk.BroadcastStatusEvent, len(m.externalEvents))
	copy(events, m.externalEvents)
	return events
}

func (m *MockStorage) GetKeyValue(_ context.Context, key string) ([]byte, bool, error) {
	m.GetKeyValueCalled.Add(1)

	m.mu.Lock()
	defer m.mu.Unlock()

	value, found := m.keyValues[key]
	if !found {
		return nil, false, nil
	}

	// return a copy so callers cannot mutate the stored value
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)
	return valueCopy, true, nil
}

func (m *MockStorage) SetKeyValue(_ context.Context, key string, value []byte) error {
	m.SetKeyValueCalled.Add(1)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.keyValues == nil {
		m.keyValues = make(map[string][]byte)
	}
	m.keyValues[key] = value
	return nil
}
