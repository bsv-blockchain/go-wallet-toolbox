package chaintracksclient_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	"github.com/bsv-blockchain/go-sdk/block"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracksclient"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracksclient/testabilities"
)

const currentHeight = uint32(918934)

func TestService_Lifecycle(t *testing.T) {
	// given:
	mockCT := testabilities.NewMockChaintracks()

	genesisHashStr := "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f"
	genesisHash, _ := chainhash.NewHashFromHex(genesisHashStr)
	genesisMerkleRoot, _ := chainhash.NewHashFromHex("4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b")

	tipHash, _ := chainhash.NewHashFromHex("00000000000000000165924d2b7e41fd586d88e02f846ea6428d37c51f97db31")

	genesisHeader := &chaintracks.BlockHeader{
		Header: &block.Header{MerkleRoot: *genesisMerkleRoot},
		Height: 0,
		Hash:   *genesisHash,
	}

	tipHeader := &chaintracks.BlockHeader{
		Header: &block.Header{},
		Height: currentHeight,
		Hash:   *tipHash,
	}

	mockCT.
		SetHeight(currentHeight).
		SetTip(tipHeader).
		SetNetwork("mainnet").
		AddHeader(genesisHeader).
		AddHeader(tipHeader)

	for i := uint32(100); i < 105; i++ {
		h, _ := chainhash.NewHashFromHex("0000000000000000000000000000000000000000000000000000000000000000")
		mockCT.AddHeader(&chaintracks.BlockHeader{
			Height: i,
			Hash:   *h,
		})
	}

	service, err := chaintracksclient.New(
		logging.NewTestLogger(t),
		nil,
		chaintracksclient.WithChaintracks(mockCT),
	)
	require.NoError(t, err, "failed to create service")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Track callback invocations
	var tipReceived atomic.Bool
	var receivedHeader atomic.Pointer[chaintracks.BlockHeader]

	// when:
	err = service.Start(ctx, chaintracksclient.Callbacks{
		OnTip: func(header *chaintracks.BlockHeader) error {
			t.Logf("Received tip: height=%d hash=%s", header.Height, header.Hash.String())
			receivedHeader.Store(header)
			tipReceived.Store(true)
			return nil
		},
	})
	require.NoError(t, err, "start should not return error")

	mockCT.SendTip(tipHeader)

	// then:
	require.Eventually(t, func() bool {
		return tipReceived.Load()
	}, 1*time.Second, 10*time.Millisecond, "should receive tip")

	header := receivedHeader.Load()
	require.NotNil(t, header, "received header should not be nil")
	assert.Equal(t, currentHeight, header.Height)

	t.Run("GetHeight", func(t *testing.T) {
		// when:
		height := service.GetHeight(ctx)

		// then:
		assert.Equal(t, currentHeight, height)
	})

	t.Run("GetTip", func(t *testing.T) {
		// when:
		tip, _ := service.GetTip(ctx)

		// then:
		require.NotNil(t, tip)
		assert.Equal(t, uint(currentHeight), tip.Height)
	})

	t.Run("CurrentHeight", func(t *testing.T) {
		// when:
		height, err := service.CurrentHeight(ctx)

		// then:
		require.NoError(t, err)
		assert.Equal(t, currentHeight, height)
	})

	t.Run("GetHeaderByHeight_genesis", func(t *testing.T) {
		// when:
		header, err := service.GetHeaderByHeight(ctx, 0)

		// then:
		require.NoError(t, err)
		require.NotNil(t, header)
		assert.Equal(t, uint(0), header.Height)
		assert.Equal(t, genesisHash.String(), header.Hash)
	})

	t.Run("GetHeaderByHeight_tip", func(t *testing.T) {
		// when:
		header, err := service.GetHeaderByHeight(ctx, currentHeight)

		// then:
		require.NoError(t, err)
		require.NotNil(t, header)
		assert.Equal(t, uint(currentHeight), header.Height)
	})

	t.Run("GetHeaderByHeight_notFound", func(t *testing.T) {
		// when:
		header, err := service.GetHeaderByHeight(ctx, 999999)

		// then:
		require.Error(t, err)
		require.Nil(t, header)
	})

	t.Run("GetHeaders", func(t *testing.T) {
		// when:
		headers, err := service.GetHeaders(ctx, 100, 5)

		// then:
		require.NoError(t, err)
		require.Len(t, headers, 5)
		for i, h := range headers {
			assert.Equal(t, uint32(100+i), h.Height)
		}
	})

	t.Run("GetNetwork", func(t *testing.T) {
		// when:
		network, err := service.GetNetwork(ctx)

		// then:
		require.NoError(t, err)
		assert.Equal(t, "mainnet", network)
	})

	t.Run("IsValidRootForHeight", func(t *testing.T) {
		// when:
		isValid, err := service.IsValidRootForHeight(ctx, genesisMerkleRoot, 0)

		// then:
		require.NoError(t, err)
		assert.True(t, isValid)

		// when:
		wrongRoot, _ := chainhash.NewHashFromHex("0000000000000000000000000000000000000000000000000000000000000001")
		isValid, err = service.IsValidRootForHeight(ctx, wrongRoot, 0)

		// then:
		require.NoError(t, err)
		assert.False(t, isValid)
	})

	t.Run("GetHeaderByHash", func(t *testing.T) {
		// when:
		header, err := service.GetHeaderByHash(ctx, genesisHashStr)

		// then:
		require.NoError(t, err)
		require.NotNil(t, header)
		assert.Equal(t, uint(0), header.Height)
		assert.Equal(t, header.Hash, genesisHashStr)
	})
}

func TestService_ReorgCallback(t *testing.T) {
	// given:
	mockCT := testabilities.NewMockChaintracks()

	reorgEvent := &chaintracks.ReorgEvent{
		Depth:          4,
		OrphanedHashes: []chainhash.Hash{{0x01}, {0x02}},
		NewTip:         &chaintracks.BlockHeader{Height: 102},
		CommonAncestor: &chaintracks.BlockHeader{Height: 100},
	}

	service, err := chaintracksclient.New(
		logging.NewTestLogger(t),
		nil,
		chaintracksclient.WithChaintracks(mockCT),
	)
	require.NoError(t, err, "failed to create service")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Track callback invocations
	var reorgReceived atomic.Bool
	var receivedReorgEvent atomic.Pointer[chaintracks.ReorgEvent]

	// when:
	err = service.Start(ctx, chaintracksclient.Callbacks{
		OnReorg: func(event *chaintracks.ReorgEvent) error {
			receivedReorgEvent.Store(event)
			reorgReceived.Store(true)
			return nil
		},
	})
	require.NoError(t, err, "start should not return error")

	mockCT.SendReorg(reorgEvent)

	// then:
	require.Eventually(t, func() bool {
		return reorgReceived.Load()
	}, 1*time.Second, 10*time.Millisecond, "should receive reorg event")

	event := receivedReorgEvent.Load()
	require.NotNil(t, event, "received reorg event should not be nil")
	assert.Equal(t, reorgEvent, event)
}

func TestService_StartWithNilCallback(t *testing.T) {
	// given:
	mockCT := testabilities.NewMockChaintracks()

	service, err := chaintracksclient.New(
		logging.NewTestLogger(t),
		nil,
		chaintracksclient.WithChaintracks(mockCT),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// when:
	err = service.Start(ctx, chaintracksclient.Callbacks{
		OnTip:   nil,
		OnReorg: nil,
	})

	// then:
	require.NoError(t, err, "start with nil callback should not return error")
}

func TestService_OnTipCallbackError(t *testing.T) {
	// given:
	mockCT := testabilities.NewMockChaintracks()
	tipHash, _ := chainhash.NewHashFromHex("00000000000000000165924d2b7e41fd586d88e02f846ea6428d37c51f97db31")
	tipHeader := &chaintracks.BlockHeader{
		Height: currentHeight,
		Hash:   *tipHash,
	}

	service, err := chaintracksclient.New(
		logging.NewTestLogger(t),
		nil,
		chaintracksclient.WithChaintracks(mockCT),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())

	var callbackInvoked atomic.Bool

	// when:
	err = service.Start(ctx, chaintracksclient.Callbacks{
		OnTip: func(header *chaintracks.BlockHeader) error {
			callbackInvoked.Store(true)
			return assert.AnError
		},
	})
	require.NoError(t, err)

	mockCT.SendTip(tipHeader)

	// then:
	require.Eventually(t, func() bool {
		return callbackInvoked.Load()
	}, 1*time.Second, 10*time.Millisecond, "callback should be invoked even if it returns error")

	// Cancel context and wait briefly for the goroutine to finish logging the error,
	// otherwise the test logger (backed by t) panics on write-after-completion.
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestService_OnReorgCallbackError(t *testing.T) {
	// given:
	mockCT := testabilities.NewMockChaintracks()
	reorgEvent := &chaintracks.ReorgEvent{
		Depth:          4,
		OrphanedHashes: []chainhash.Hash{{0x01}, {0x02}},
		NewTip:         &chaintracks.BlockHeader{Height: 102},
		CommonAncestor: &chaintracks.BlockHeader{Height: 100},
	}

	service, err := chaintracksclient.New(
		logging.NewTestLogger(t),
		nil,
		chaintracksclient.WithChaintracks(mockCT),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())

	var callbackInvoked atomic.Bool

	// when:
	err = service.Start(ctx, chaintracksclient.Callbacks{
		OnReorg: func(event *chaintracks.ReorgEvent) error {
			callbackInvoked.Store(true)
			return assert.AnError
		},
	})
	require.NoError(t, err)

	mockCT.SendReorg(reorgEvent)

	// then:
	require.Eventually(t, func() bool {
		return callbackInvoked.Load()
	}, 1*time.Second, 10*time.Millisecond, "callback should be invoked even if it returns error")

	// Cancel context and wait briefly for the goroutine to finish logging the error,
	// otherwise the test logger (backed by t) panics on write-after-completion.
	cancel()
	time.Sleep(50 * time.Millisecond)
}
