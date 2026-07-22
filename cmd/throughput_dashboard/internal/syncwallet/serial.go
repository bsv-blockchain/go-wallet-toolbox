// Package syncwallet serializes wallet RPC through a single mutex.
//
// The storage HTTP client uses BRC-104 AuthFetch, which is not safe for concurrent
// session handshakes on one peer. FuelKeeper + metrics sampler both call ListOutputs
// at startup and can deadlock for minutes, leaving the dashboard UI at empty ticks
// even when funds are already internalized.
package syncwallet

import (
	"context"
	"sync"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Wallet is the operator surface used by the dashboard (stream, sampler, keeper, funding).
type Wallet interface {
	Balance(ctx context.Context) (uint64, error)
	ListOutputs(ctx context.Context, args sdk.ListOutputsArgs, originator string) (*sdk.ListOutputsResult, error)
	CreateAction(ctx context.Context, args sdk.CreateActionArgs, originator string) (*sdk.CreateActionResult, error)
	FanOutFuel(ctx context.Context, shape wdk.ShapedChange, originator string) (*sdk.CreateActionResult, error)
	InternalizeAction(ctx context.Context, args sdk.InternalizeActionArgs, originator string) (*sdk.InternalizeActionResult, error)
	Close()
}

// Serial wraps Wallet with a mutex so only one storage RPC runs at a time.
type Serial struct {
	mu sync.Mutex
	w  Wallet
}

// New returns a serializing wrapper around w.
func New(w Wallet) *Serial {
	return &Serial{w: w}
}

// Unwrap returns the underlying wallet (for Close).
func (s *Serial) Unwrap() Wallet { return s.w }

func (s *Serial) Balance(ctx context.Context) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Balance(ctx)
}

func (s *Serial) ListOutputs(ctx context.Context, args sdk.ListOutputsArgs, originator string) (*sdk.ListOutputsResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.ListOutputs(ctx, args, originator)
}

func (s *Serial) CreateAction(ctx context.Context, args sdk.CreateActionArgs, originator string) (*sdk.CreateActionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.CreateAction(ctx, args, originator)
}

func (s *Serial) FanOutFuel(ctx context.Context, shape wdk.ShapedChange, originator string) (*sdk.CreateActionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.FanOutFuel(ctx, shape, originator)
}

func (s *Serial) InternalizeAction(ctx context.Context, args sdk.InternalizeActionArgs, originator string) (*sdk.InternalizeActionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.InternalizeAction(ctx, args, originator)
}

func (s *Serial) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.w.Close()
}
