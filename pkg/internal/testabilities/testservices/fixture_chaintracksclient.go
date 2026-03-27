package testservices

import (
	"errors"
	"testing"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	"github.com/bsv-blockchain/go-sdk/block"
	"github.com/bsv-blockchain/go-sdk/chainhash"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracksclient"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracksclient/testabilities"
)

const defaultChaintracksHeight = uint32(800000)

// ChaintracksClientFixture provides test utilities for mocking chaintracks behavior.
// Unlike HTTP-based fixtures, chaintracks uses a direct interface mock.
// Error scenarios are handled by not setting data - the mock returns ErrHeaderNotFound for missing headers,
// or by using WillFail() to force all methods to return an error.
type ChaintracksClientFixture interface {
	IsUpAndRunning() ChaintracksClientFixture
	WillFail() error
	SetHeight(height uint32) ChaintracksClientFixture
	SetTip(header *chaintracks.BlockHeader) ChaintracksClientFixture
	AddHeader(header *chaintracks.BlockHeader) ChaintracksClientFixture
	Adapter() *chaintracksclient.Adapter
}

type chaintracksClientFixture struct {
	t       testing.TB
	mock    *testabilities.MockChaintracks
	adapter *chaintracksclient.Adapter
}

// NewChaintracksClientFixture creates a new chaintracks test fixture.
func NewChaintracksClientFixture(t testing.TB) ChaintracksClientFixture {
	mock := testabilities.NewMockChaintracks()

	adapter, err := chaintracksclient.New(
		logging.NewTestLogger(t),
		nil,
		chaintracksclient.WithChaintracks(mock),
	)
	if err != nil {
		t.Fatalf("failed to create chaintracks adapter: %v", err)
	}

	return &chaintracksClientFixture{
		t:       t,
		mock:    mock,
		adapter: adapter,
	}
}

func (f *chaintracksClientFixture) SetHeight(height uint32) ChaintracksClientFixture {
	f.t.Helper()
	f.mock.SetHeight(height)
	return f
}

func (f *chaintracksClientFixture) SetTip(header *chaintracks.BlockHeader) ChaintracksClientFixture {
	f.t.Helper()
	f.mock.SetTip(header)
	return f
}

func (f *chaintracksClientFixture) AddHeader(header *chaintracks.BlockHeader) ChaintracksClientFixture {
	f.t.Helper()
	f.mock.AddHeader(header)
	return f
}

func (f *chaintracksClientFixture) IsUpAndRunning() ChaintracksClientFixture {
	f.t.Helper()

	f.mock.SetHeight(defaultChaintracksHeight)

	defaultTip := defaultBlockHeader(defaultChaintracksHeight)
	f.mock.SetTip(defaultTip)
	f.mock.AddHeader(defaultTip)

	return f
}

// ErrChaintracksForcedFailure is the error returned when WillFail() is called.
var ErrChaintracksForcedFailure = errors.New("chaintracks: forced test failure")

func (f *chaintracksClientFixture) WillFail() error {
	f.t.Helper()
	f.mock.SetError(ErrChaintracksForcedFailure)
	return ErrChaintracksForcedFailure
}

func (f *chaintracksClientFixture) Adapter() *chaintracksclient.Adapter {
	return f.adapter
}

func defaultBlockHeader(height uint32) *chaintracks.BlockHeader {
	return &chaintracks.BlockHeader{
		Header: &block.Header{
			Version:    1,
			PrevHash:   chainhash.Hash{},
			MerkleRoot: chainhash.Hash{1, 2, 3, 4},
			Timestamp:  1234567890,
			Bits:       0x1d00ffff,
			Nonce:      0,
		},
		Height: height,
		Hash:   chainhash.Hash{0xaa, 0xbb, 0xcc},
	}
}
