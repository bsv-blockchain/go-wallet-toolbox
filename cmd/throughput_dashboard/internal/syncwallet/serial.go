// Package syncwallet bounds concurrent wallet RPC through a fixed slot pool.
//
// History: the storage HTTP client's BRC-104 AuthFetch (go-sdk v1.3.1) was not
// safe for concurrent requests — its response listener deregistered itself on
// the FIRST response to arrive, so other in-flight requests hung until the 30s
// timeout, and the server's session manager had a remove-then-add window that
// dropped sessions under concurrency. Both are fixed in the locally patched
// SDK (see third_party/go-sdk, go.mod replace), so requests may now run
// concurrently on one client. The pool still exists for two reasons:
//
//   - The local infra HTTP server starts failing connections (EOF) somewhere
//     past ~16-32 concurrent requests; MaxInFlight keeps the demo inside the
//     window the stack actually sustains.
//   - Waiting respects context cancellation and blocked senders wake FIFO, so
//     a burst from one component (FuelKeeper catch-up) cannot starve the
//     metrics sampler or stream workers that queued first.
package syncwallet

import (
	"context"
	"fmt"
	"sync"
	"time"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// DefaultMaxInFlight is the slot count used when New receives n <= 0.
// 32 measured as the throughput sweet spot on the local stack (~460
// createActions/s): 16 leaves the server idle, while 64 collapses into DB
// lock contention (~270/s real). Tune per stack via WALLET_MAX_IN_FLIGHT.
const DefaultMaxInFlight = 32

// closeWait bounds how long Close waits to drain in-flight RPCs before
// closing the underlying wallet anyway: shutdown must stay inside docker's
// ~10s grace window even when a storage RPC is wedged.
const closeWait = 3 * time.Second

// Wallet is the operator surface used by the dashboard (stream, sampler, keeper, funding).
type Wallet interface {
	Balance(ctx context.Context) (uint64, error)
	ListOutputs(ctx context.Context, args sdk.ListOutputsArgs, originator string) (*sdk.ListOutputsResult, error)
	ListActions(ctx context.Context, args sdk.ListActionsArgs, originator string) (*sdk.ListActionsResult, error)
	CreateAction(ctx context.Context, args sdk.CreateActionArgs, originator string) (*sdk.CreateActionResult, error)
	FanOutFuel(ctx context.Context, shape wdk.ShapedChange, originator string) (*sdk.CreateActionResult, error)
	InternalizeAction(ctx context.Context, args sdk.InternalizeActionArgs, originator string) (*sdk.InternalizeActionResult, error)
	Close()
}

// Bounded wraps Wallet so at most maxInFlight storage RPCs run concurrently.
type Bounded struct {
	slots     chan struct{}
	w         Wallet
	closeOnce sync.Once
}

// New returns a concurrency-bounding wrapper around w with n slots
// (n <= 0 → DefaultMaxInFlight).
func New(w Wallet, n int) *Bounded {
	if n <= 0 {
		n = DefaultMaxInFlight
	}
	return &Bounded{slots: make(chan struct{}, n), w: w}
}

// Unwrap returns the underlying wallet (for Close).
func (s *Bounded) Unwrap() Wallet { return s.w }

// acquire takes an RPC slot, or fails when ctx ends first.
func (s *Bounded) acquire(ctx context.Context) error {
	// Don't let a select race grant a slot to an already-dead context.
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("bounded wallet: context ended before RPC slot acquired: %w", err)
	}
	select {
	case s.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("bounded wallet: context ended while waiting for RPC slot: %w", ctx.Err())
	}
}

func (s *Bounded) release() { <-s.slots }

func (s *Bounded) Balance(ctx context.Context) (uint64, error) {
	if err := s.acquire(ctx); err != nil {
		return 0, err
	}
	defer s.release()
	return s.w.Balance(ctx)
}

func (s *Bounded) ListOutputs(ctx context.Context, args sdk.ListOutputsArgs, originator string) (*sdk.ListOutputsResult, error) {
	if err := s.acquire(ctx); err != nil {
		return nil, err
	}
	defer s.release()
	return s.w.ListOutputs(ctx, args, originator)
}

func (s *Bounded) ListActions(ctx context.Context, args sdk.ListActionsArgs, originator string) (*sdk.ListActionsResult, error) {
	if err := s.acquire(ctx); err != nil {
		return nil, err
	}
	defer s.release()
	return s.w.ListActions(ctx, args, originator)
}

func (s *Bounded) CreateAction(ctx context.Context, args sdk.CreateActionArgs, originator string) (*sdk.CreateActionResult, error) {
	if err := s.acquire(ctx); err != nil {
		return nil, err
	}
	defer s.release()
	return s.w.CreateAction(ctx, args, originator)
}

func (s *Bounded) FanOutFuel(ctx context.Context, shape wdk.ShapedChange, originator string) (*sdk.CreateActionResult, error) {
	if err := s.acquire(ctx); err != nil {
		return nil, err
	}
	defer s.release()
	return s.w.FanOutFuel(ctx, shape, originator)
}

func (s *Bounded) InternalizeAction(ctx context.Context, args sdk.InternalizeActionArgs, originator string) (*sdk.InternalizeActionResult, error) {
	if err := s.acquire(ctx); err != nil {
		return nil, err
	}
	defer s.release()
	return s.w.InternalizeAction(ctx, args, originator)
}

// Close closes the underlying wallet once. It waits up to closeWait for
// in-flight RPCs to drain (they cannot be canceled), then closes anyway so a
// wedged RPC cannot hang process shutdown. Slots are never released after
// Close begins, so post-Close RPCs queue (and fail on their own contexts)
// instead of hitting a closed wallet.
func (s *Bounded) Close() {
	s.closeOnce.Do(func() {
		deadline := time.After(closeWait)
		for i := 0; i < cap(s.slots); i++ {
			select {
			case s.slots <- struct{}{}:
			case <-deadline:
				s.w.Close()
				return
			}
		}
		s.w.Close()
	})
}
