package testabilities

import (
	"sync"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type MockHeadersReceiver struct {
	RespChan        chan wdk.ChainBlockHeader
	ReceivedHeaders []wdk.ChainBlockHeader
	Locker          sync.Mutex
}

func NewMockHeadersReceiver(t testing.TB) *MockHeadersReceiver {
	receiver := &MockHeadersReceiver{
		RespChan:        make(chan wdk.ChainBlockHeader, 1),
		ReceivedHeaders: make([]wdk.ChainBlockHeader, 0),
	}

	t.Cleanup(func() {
		close(receiver.RespChan)
	})

	go func() {
		for headers := range receiver.RespChan {
			receiver.Locker.Lock()
			receiver.ReceivedHeaders = append(receiver.ReceivedHeaders, headers)
			receiver.Locker.Unlock()
		}
	}()

	return receiver
}

func (m *MockHeadersReceiver) GetReceivedHeaders() []wdk.ChainBlockHeader {
	m.Locker.Lock()
	defer m.Locker.Unlock()
	return m.ReceivedHeaders
}
