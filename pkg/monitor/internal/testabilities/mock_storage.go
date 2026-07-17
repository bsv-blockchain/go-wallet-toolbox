package testabilities

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// CannedSendWaitingTxID is the txID carried by the canned SendWaitingTransactions result below, so
// tests asserting the SendWaiting task's channel forwarding can match the emitted message.
const CannedSendWaitingTxID = "0000000000000000000000000000000000000000000000000000000000000abc"

// CannedSendWaitingResult is returned by MockStorage.SendWaitingTransactions. It carries a single
// NotDelayedResults entry so the SendWaiting monitor task has something to forward onto the
// TxBroadcasted channel (previously the storage layer always returned nil, so the forwarding path
// was dead code that no test could exercise).
var CannedSendWaitingResult = &wdk.ProcessActionResult{
	NotDelayedResults: []wdk.ReviewActionResult{
		{
			TxID:      primitives.TXIDHexString(CannedSendWaitingTxID),
			Status:    wdk.ReviewActionResultStatusSuccess,
			Reference: "canned-reference",
			Labels:    []string{"canned-label"},
		},
	},
}

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
	return CannedSendWaitingResult, nil
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
