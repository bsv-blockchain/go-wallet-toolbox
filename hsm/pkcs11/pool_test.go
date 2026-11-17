//go:build pkcs11

package pkcs11

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPoolAcquireCancel(t *testing.T) {
	p := newPool(nil, 1)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := p.acquire(ctx)
	require.Error(t, err)
}

func TestPoolReleaseAfterShutdown(t *testing.T) {
	p := newPool(nil, 1)
	session := &sess{pool: p}
	p.add(session)
	got, err := p.acquire(t.Context())
	require.NoError(t, err)

	released := make(chan struct{})
	go func() {
		p.release(got)
		close(released)
	}()
	done := make(chan struct{})
	go func() {
		p.shutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown hung")
	}
	select {
	case <-released:
	case <-time.After(2 * time.Second):
		t.Fatal("release hung")
	}
	p.release(got)
}

func TestPoolShutdownWhileBusy(t *testing.T) {
	for range 200 {
		p := newPool(nil, 4)
		for range 4 {
			p.add(&sess{pool: p})
		}
		var wg sync.WaitGroup
		stop := make(chan struct{})
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
					}
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
					session, err := p.acquire(ctx)
					cancel()
					if err != nil {
						continue
					}
					p.release(session)
				}
			}()
		}
		time.Sleep(time.Millisecond)
		done := make(chan struct{})
		go func() {
			p.shutdown()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("shutdown hung while sessions were in flight")
		}
		close(stop)
		wg.Wait()
	}
}
