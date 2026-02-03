package services

import (
	"log/slog"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	"github.com/bsv-blockchain/go-sdk/block"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTipBroadcaster_Subscribe(t *testing.T) {
	// given:
	broadcaster := newTipBroadcaster(slog.Default())

	// when:
	ch, unsub := broadcaster.Subscribe()

	// then:
	require.NotNil(t, ch)
	require.NotNil(t, unsub)

	// cleanup:
	unsub()
}

func TestTipBroadcaster_BroadcastToSingleSubscriber(t *testing.T) {
	// given:
	broadcaster := newTipBroadcaster(slog.Default())
	ch, unsub := broadcaster.Subscribe()
	defer unsub()

	testHeader := createTestBlockHeader(100, "0000000000000000000000000000000000000000000000000000000000000001")

	// when:
	broadcaster.broadcast(testHeader)

	// then:
	select {
	case received := <-ch:
		assert.Equal(t, testHeader.Height, received.Height)
		assert.Equal(t, testHeader.Hash, received.Hash)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected to receive header but timed out")
	}
}

func TestTipBroadcaster_BroadcastToMultipleSubscribers(t *testing.T) {
	// given:
	broadcaster := newTipBroadcaster(slog.Default())

	ch1, unsub1 := broadcaster.Subscribe()
	defer unsub1()
	ch2, unsub2 := broadcaster.Subscribe()
	defer unsub2()
	ch3, unsub3 := broadcaster.Subscribe()
	defer unsub3()

	testHeader := createTestBlockHeader(200, "0000000000000000000000000000000000000000000000000000000000000002")

	// when:
	broadcaster.broadcast(testHeader)

	// then:
	for i, ch := range []<-chan *chaintracks.BlockHeader{ch1, ch2, ch3} {
		select {
		case received := <-ch:
			assert.Equal(t, testHeader.Height, received.Height, "subscriber %d", i+1)
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("subscriber %d: expected to receive header but timed out", i+1)
		}
	}
}

func TestTipBroadcaster_UnsubscribeStopsReceiving(t *testing.T) {
	// given:
	broadcaster := newTipBroadcaster(slog.Default())
	ch, unsub := broadcaster.Subscribe()

	// when:
	unsub()

	// and: broadcast after unsubscribe
	testHeader := createTestBlockHeader(300, "0000000000000000000000000000000000000000000000000000000000000003")
	broadcaster.broadcast(testHeader)

	// then:
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed after unsubscribe")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected channel to be closed but it wasn't")
	}
}

func createTestBlockHeader(height uint32, hashStr string) *chaintracks.BlockHeader {
	hash, _ := chainhash.NewHashFromHex(hashStr)
	merkleRoot, _ := chainhash.NewHashFromHex("0000000000000000000000000000000000000000000000000000000000000000")
	prevHash, _ := chainhash.NewHashFromHex("0000000000000000000000000000000000000000000000000000000000000000")

	header := &block.Header{
		Version:    1,
		PrevHash:   *prevHash,
		MerkleRoot: *merkleRoot,
		Timestamp:  1234567890,
		Bits:       0x1d00ffff,
		Nonce:      0,
	}

	return &chaintracks.BlockHeader{
		Height: height,
		Hash:   *hash,
		Header: header,
	}
}
