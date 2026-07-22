package metrics

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/stream"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeWallet implements WalletAPI with controllable balance and basket totals.
type fakeWallet struct {
	mu           sync.Mutex
	balance      uint64
	balanceErr   error
	fuelTotal    uint32
	reserveTotal uint32
	listErr      error
	listCalls    []sdk.ListOutputsArgs
	originators  []string
}

func (f *fakeWallet) Balance(context.Context) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.balanceErr != nil {
		return 0, f.balanceErr
	}
	return f.balance, nil
}

func (f *fakeWallet) ListOutputs(_ context.Context, args sdk.ListOutputsArgs, originator string) (*sdk.ListOutputsResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls = append(f.listCalls, args)
	f.originators = append(f.originators, originator)
	if f.listErr != nil {
		return nil, f.listErr
	}
	var total uint32
	switch args.Basket {
	case wdk.BasketNameForFuel:
		total = f.fuelTotal
	case wdk.BasketNameForReserve:
		total = f.reserveTotal
	}
	return &sdk.ListOutputsResult{TotalOutputs: total}, nil
}

func (f *fakeWallet) setFuel(n uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fuelTotal = n
}

func (f *fakeWallet) setReserve(n uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reserveTotal = n
}

// fakeActionCreator satisfies stream.ActionCreator for constructing a Controller.
type fakeActionCreator struct {
	calls atomic.Uint64
	fail  bool
}

func (f *fakeActionCreator) CreateAction(ctx context.Context, _ sdk.CreateActionArgs, _ string) (*sdk.CreateActionResult, error) {
	f.calls.Add(1)
	if f.fail {
		return nil, errors.New("createAction failed")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return &sdk.CreateActionResult{}, nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestSampler(t *testing.T, wallet WalletAPI, ctrl *stream.Controller, targetTPS uint64) *Sampler {
	t.Helper()
	if ctrl == nil {
		ctrl = stream.NewController(&fakeActionCreator{}, stream.Options{TPS: 1, Workers: 1, Originator: "test"}, discardLogger())
	}
	// long interval; tests drive sample() directly
	return NewSampler(wallet, ctrl, Config{
		Originator:       "test-originator",
		Interval:         time.Hour,
		TargetTPS:        targetTPS,
		Denomination:     240,
		TargetPool:       1000,
		LowWaterPercent:  60,
		HighWaterPercent: 100,
		Logger:           discardLogger(),
	})
}

func TestNewSampler_Defaults(t *testing.T) {
	ctrl := stream.NewController(&fakeActionCreator{}, stream.Options{}, discardLogger())
	s := NewSampler(&fakeWallet{}, ctrl, Config{
		Originator: "orig", Interval: 0,
		TargetTPS: 10, Denomination: 240, TargetPool: 1000,
		LowWaterPercent: 60, HighWaterPercent: 100,
		Logger: nil,
	})

	assert.Equal(t, time.Second, s.interval, "non-positive interval defaults to 1s")
	assert.Equal(t, uint64(600), s.lowWater)
	assert.Equal(t, uint64(1000), s.highWater)
	assert.Equal(t, 200, s.maxEvents)
	assert.NotNil(t, s.logger)
	assert.NotNil(t, s.subscribers)
}

func TestSample_TickFieldsAndRunway(t *testing.T) {
	wallet := &fakeWallet{balance: 50_000, fuelTotal: 120, reserveTotal: 40}
	s := newTestSampler(t, wallet, nil, 10)

	s.sample(context.Background())

	tick := s.LastTick()
	assert.NotEmpty(t, tick.Timestamp)
	assert.Equal(t, uint64(50_000), tick.DefaultSats)
	assert.Equal(t, uint64(120), tick.FuelCount)
	assert.Equal(t, uint64(40), tick.ReserveCount)
	assert.InDelta(t, 12.0, tick.FuelRunwaySec, 1e-9) // 120 / 10
	assert.Equal(t, uint64(10), tick.TargetTPS)
	assert.Equal(t, uint64(240), tick.Denomination)
	assert.Equal(t, uint64(600), tick.LowWater)
	assert.Equal(t, uint64(1000), tick.HighWater)
	assert.Equal(t, uint64(1000), tick.TargetPoolSize)
	assert.Equal(t, uint64(0), tick.TPSAttempted)
	assert.Equal(t, uint64(0), tick.TPSSucceeded)
	assert.Equal(t, uint64(0), tick.TPSFailed)

	// ListOutputs polled for fuel + reserve with limit 1 and originator.
	require.Len(t, wallet.listCalls, 2)
	assert.Equal(t, wdk.BasketNameForFuel, wallet.listCalls[0].Basket)
	assert.Equal(t, wdk.BasketNameForReserve, wallet.listCalls[1].Basket)
	require.NotNil(t, wallet.listCalls[0].Limit)
	assert.Equal(t, uint32(1), *wallet.listCalls[0].Limit)
	assert.Equal(t, []string{"test-originator", "test-originator"}, wallet.originators)
}

func TestSample_RunwayZeroWhenTargetTPSZero(t *testing.T) {
	wallet := &fakeWallet{fuelTotal: 500}
	s := newTestSampler(t, wallet, nil, 0)

	s.sample(context.Background())
	assert.InDelta(t, 0, s.LastTick().FuelRunwaySec, 0.001)
}

func TestSample_EmitsTickEvent(t *testing.T) {
	s := newTestSampler(t, &fakeWallet{fuelTotal: 1}, nil, 5)
	s.sample(context.Background())

	events := s.RecentEvents()
	require.Len(t, events, 1)
	assert.Equal(t, "tick", events[0].Type)
	assert.NotEmpty(t, events[0].Timestamp)
	require.Contains(t, events[0].Payload, "tick")
}

func TestSample_NoTopupOnFirstSample(t *testing.T) {
	wallet := &fakeWallet{fuelTotal: 100, reserveTotal: 50}
	s := newTestSampler(t, wallet, nil, 10)

	s.sample(context.Background())

	for _, ev := range s.RecentEvents() {
		assert.NotEqual(t, "topup", ev.Type, "first sample must not emit topup")
	}
}

func TestSample_TopupOnFuelIncrease(t *testing.T) {
	wallet := &fakeWallet{fuelTotal: 10, reserveTotal: 5}
	s := newTestSampler(t, wallet, nil, 10)

	s.sample(context.Background())
	wallet.setFuel(25)
	s.sample(context.Background())

	var topups []Event
	for _, ev := range s.RecentEvents() {
		if ev.Type == "topup" {
			topups = append(topups, ev)
		}
	}
	require.Len(t, topups, 1)
	assert.Equal(t, wdk.BasketNameForFuel, topups[0].Payload["basket"])
	assert.Equal(t, uint64(10), topups[0].Payload["before"])
	assert.Equal(t, uint64(25), topups[0].Payload["after"])
	assert.Equal(t, uint64(15), topups[0].Payload["delta"])
	assert.Equal(t, "fuel_inventory_increase", topups[0].Payload["kind"])
}

func TestSample_TopupOnReserveIncrease(t *testing.T) {
	wallet := &fakeWallet{fuelTotal: 10, reserveTotal: 5}
	s := newTestSampler(t, wallet, nil, 10)

	s.sample(context.Background())
	wallet.setReserve(12)
	s.sample(context.Background())

	var topups []Event
	for _, ev := range s.RecentEvents() {
		if ev.Type == "topup" {
			topups = append(topups, ev)
		}
	}
	require.Len(t, topups, 1)
	assert.Equal(t, wdk.BasketNameForReserve, topups[0].Payload["basket"])
	assert.Equal(t, uint64(5), topups[0].Payload["before"])
	assert.Equal(t, uint64(12), topups[0].Payload["after"])
	assert.Equal(t, uint64(7), topups[0].Payload["delta"])
	assert.Equal(t, "reserve_inventory_increase", topups[0].Payload["kind"])
}

func TestSample_TopupBothBaskets(t *testing.T) {
	wallet := &fakeWallet{fuelTotal: 1, reserveTotal: 1}
	s := newTestSampler(t, wallet, nil, 10)

	s.sample(context.Background())
	wallet.setFuel(2)
	wallet.setReserve(3)
	s.sample(context.Background())

	var kinds []string
	for _, ev := range s.RecentEvents() {
		if ev.Type == "topup" {
			kinds = append(kinds, ev.Payload["kind"].(string))
		}
	}
	assert.ElementsMatch(t, []string{"fuel_inventory_increase", "reserve_inventory_increase"}, kinds)
}

func TestSample_NoTopupOnDecreaseOrUnchanged(t *testing.T) {
	wallet := &fakeWallet{fuelTotal: 20, reserveTotal: 20}
	s := newTestSampler(t, wallet, nil, 10)

	s.sample(context.Background())
	wallet.setFuel(15)    // decrease
	wallet.setReserve(20) // unchanged
	s.sample(context.Background())

	for _, ev := range s.RecentEvents() {
		assert.NotEqual(t, "topup", ev.Type)
	}
	// two ticks only
	assert.Len(t, s.RecentEvents(), 2)
}

func TestSample_StreamDeltasAfterActivity(t *testing.T) {
	actions := &fakeActionCreator{}
	ctrl := stream.NewController(actions, stream.Options{TPS: 200, Workers: 4, Originator: "test"}, discardLogger())
	wallet := &fakeWallet{}
	s := newTestSampler(t, wallet, ctrl, 10)

	// Baseline sample with idle stream.
	s.sample(context.Background())
	require.Equal(t, uint64(0), s.LastTick().TPSAttempted)

	require.NoError(t, ctrl.Start(context.Background(), stream.Options{}))
	time.Sleep(80 * time.Millisecond)
	ctrl.Stop()

	s.sample(context.Background())
	tick := s.LastTick()
	require.Positive(t, tick.TPSAttempted, "expected stream activity deltas after run")
	assert.Equal(t, tick.TPSAttempted, tick.TPSSucceeded+tick.TPSFailed)
	assert.Equal(t, tick.Stream.Attempted, tick.TPSAttempted) // prev was 0

	// Idle again: deltas should be zero while cumulative stats remain.
	prevAttempted := tick.Stream.Attempted
	s.sample(context.Background())
	tick2 := s.LastTick()
	assert.Equal(t, uint64(0), tick2.TPSAttempted)
	assert.Equal(t, prevAttempted, tick2.Stream.Attempted)
}

func TestSubscribe_ReceivesEvents(t *testing.T) {
	s := newTestSampler(t, &fakeWallet{fuelTotal: 1}, nil, 5)
	ch := s.Subscribe()
	defer s.Unsubscribe(ch)

	s.sample(context.Background())

	select {
	case ev := <-ch:
		assert.Equal(t, "tick", ev.Type)
	case <-time.After(time.Second):
		t.Fatal("expected tick event on subscriber channel")
	}
}

func TestSubscribe_ReceivesTopup(t *testing.T) {
	wallet := &fakeWallet{fuelTotal: 1}
	s := newTestSampler(t, wallet, nil, 5)
	ch := s.Subscribe()
	defer s.Unsubscribe(ch)

	s.sample(context.Background())
	// drain first tick
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("missing first tick")
	}

	wallet.setFuel(10)
	s.sample(context.Background())

	var gotTopup, gotTick bool
	deadline := time.After(time.Second)
	for !gotTopup || !gotTick {
		select {
		case ev := <-ch:
			switch ev.Type {
			case "topup":
				gotTopup = true
			case "tick":
				gotTick = true
			}
		case <-deadline:
			t.Fatalf("timeout waiting for topup+tick (topup=%v tick=%v)", gotTopup, gotTick)
		}
	}
}

func TestUnsubscribe_ClosesChannelAndStopsDelivery(t *testing.T) {
	s := newTestSampler(t, &fakeWallet{}, nil, 5)
	ch := s.Subscribe()
	s.Unsubscribe(ch)

	// Channel should be closed.
	_, ok := <-ch
	assert.False(t, ok, "unsubscribed channel should be closed")

	// Further samples must not panic or re-open delivery.
	s.sample(context.Background())
	assert.Len(t, s.RecentEvents(), 1)

	// Double unsubscribe is safe.
	s.Unsubscribe(ch)
}

func TestEmit_NonBlockingWhenSubscriberFull(t *testing.T) {
	s := newTestSampler(t, &fakeWallet{}, nil, 5)
	ch := s.Subscribe() // buffer 32
	defer s.Unsubscribe(ch)

	// Flood past buffer capacity; sample/emit must not block.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 64; i++ {
			s.sample(context.Background())
		}
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(3 * time.Second):
		t.Fatal("emit blocked on slow/full subscriber")
	}

	// Channel has at most buffer capacity of events.
	n := 0
drain:
	for {
		select {
		case <-ch:
			n++
		default:
			break drain
		}
	}
	assert.LessOrEqual(t, n, 32)
	assert.Positive(t, n)
}

func TestRecentEvents_RingCap(t *testing.T) {
	s := newTestSampler(t, &fakeWallet{}, nil, 5)
	s.maxEvents = 5

	for i := 0; i < 12; i++ {
		s.sample(context.Background())
	}

	events := s.RecentEvents()
	require.Len(t, events, 5)
	// All remaining should be ticks from the latest samples.
	for _, ev := range events {
		assert.Equal(t, "tick", ev.Type)
	}

	// Copy independence: mutating returned slice does not affect ring.
	events[0].Type = "mutated"
	assert.Equal(t, "tick", s.RecentEvents()[0].Type)
}

func TestRun_SamplesThenStopsOnCancel(t *testing.T) {
	wallet := &fakeWallet{balance: 1, fuelTotal: 2}
	s := NewSampler(wallet, stream.NewController(&fakeActionCreator{}, stream.Options{TPS: 1, Workers: 1}, discardLogger()), Config{
		Originator: "orig", Interval: 50 * time.Millisecond,
		TargetTPS: 10, Denomination: 240, TargetPool: 100,
		LowWaterPercent: 60, HighWaterPercent: 100,
		Logger: discardLogger(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	// Wait for at least the immediate sample + one ticker fire.
	require.Eventually(t, func() bool {
		return len(s.RecentEvents()) >= 2
	}, time.Second, 10*time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not exit after cancel")
	}

	tick := s.LastTick()
	assert.Equal(t, uint64(1), tick.DefaultSats)
	assert.Equal(t, uint64(2), tick.FuelCount)
}

func TestSample_WalletErrorsDoNotPanic(t *testing.T) {
	wallet := &fakeWallet{
		balanceErr: errors.New("balance down"),
		listErr:    errors.New("list down"),
	}
	s := newTestSampler(t, wallet, nil, 10)

	require.NotPanics(t, func() {
		s.sample(context.Background())
	})
	tick := s.LastTick()
	assert.Equal(t, uint64(0), tick.DefaultSats)
	assert.Equal(t, uint64(0), tick.FuelCount)
	assert.Equal(t, uint64(0), tick.ReserveCount)
	// still emits tick
	require.Len(t, s.RecentEvents(), 1)
}

func TestConcurrentLastTickAndSubscribe(t *testing.T) {
	wallet := &fakeWallet{fuelTotal: 5}
	s := newTestSampler(t, wallet, nil, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Run(ctx)
	}()

	// Concurrent readers + short-lived subscribers while Run samples.
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = s.LastTick()
				_ = s.RecentEvents()
				ch := s.Subscribe()
				s.Unsubscribe(ch)
			}
		}()
	}

	time.Sleep(100 * time.Millisecond)
	cancel()
	wg.Wait()
}
