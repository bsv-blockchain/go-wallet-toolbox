package testabilities

import (
	"context"
	"sync"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	"github.com/bsv-blockchain/go-sdk/chainhash"
)

// MockChaintracks is a mock implementation of the chaintracks.Chaintracks interface for testing.
type MockChaintracks struct {
	mu sync.RWMutex

	height  uint32
	tip     *chaintracks.BlockHeader
	headers map[uint32]*chaintracks.BlockHeader
	network string

	// Subscription management - use bidirectional channels internally
	subscribers []chan *chaintracks.BlockHeader
}

// NewMockChaintracks creates a new mock chaintracks instance.
func NewMockChaintracks() *MockChaintracks {
	return &MockChaintracks{
		headers: make(map[uint32]*chaintracks.BlockHeader),
		network: "testnet",
	}
}

// SetHeight sets the mock height.
func (m *MockChaintracks) SetHeight(height uint32) *MockChaintracks {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.height = height
	return m
}

// SetTip sets the mock tip.
func (m *MockChaintracks) SetTip(tip *chaintracks.BlockHeader) *MockChaintracks {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tip = tip
	return m
}

// SetNetwork sets the mock network name.
func (m *MockChaintracks) SetNetwork(network string) *MockChaintracks {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.network = network
	return m
}

// AddHeader adds a header at the specified height.
func (m *MockChaintracks) AddHeader(header *chaintracks.BlockHeader) *MockChaintracks {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.headers[header.Height] = header
	return m
}

// SendTip sends a tip update to all subscribers.
func (m *MockChaintracks) SendTip(header *chaintracks.BlockHeader) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, ch := range m.subscribers {
		select {
		case ch <- header:
		default:
		}
	}
}

func (m *MockChaintracks) GetHeight(_ context.Context) uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.height
}

func (m *MockChaintracks) GetTip(_ context.Context) *chaintracks.BlockHeader {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tip
}

func (m *MockChaintracks) GetHeaderByHeight(_ context.Context, height uint32) (*chaintracks.BlockHeader, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if header, ok := m.headers[height]; ok {
		return header, nil
	}
	return nil, chaintracks.ErrHeaderNotFound
}

func (m *MockChaintracks) GetHeaderByHash(_ context.Context, hash *chainhash.Hash) (*chaintracks.BlockHeader, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, header := range m.headers {
		if header.Hash.IsEqual(hash) {
			return header, nil
		}
	}
	return nil, chaintracks.ErrHeaderNotFound
}

func (m *MockChaintracks) GetHeaders(_ context.Context, height, count uint32) ([]*chaintracks.BlockHeader, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*chaintracks.BlockHeader
	for i := uint32(0); i < count; i++ {
		if header, ok := m.headers[height+i]; ok {
			result = append(result, header)
		}
	}
	return result, nil
}

func (m *MockChaintracks) GetNetwork(_ context.Context) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.network, nil
}

func (m *MockChaintracks) Subscribe(_ context.Context) <-chan *chaintracks.BlockHeader {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan *chaintracks.BlockHeader, 1)
	m.subscribers = append(m.subscribers, ch)
	return ch
}

func (m *MockChaintracks) Unsubscribe(ch <-chan *chaintracks.BlockHeader) {
	// No-op, for testing it doesn't need to be implemented
}

func (m *MockChaintracks) IsValidRootForHeight(_ context.Context, root *chainhash.Hash, height uint32) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if header, ok := m.headers[height]; ok {
		return header.MerkleRoot.IsEqual(root), nil
	}
	return false, chaintracks.ErrHeaderNotFound
}

func (m *MockChaintracks) CurrentHeight(_ context.Context) (uint32, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.height, nil
}
