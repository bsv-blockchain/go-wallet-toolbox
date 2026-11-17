//go:build pkcs11

package pkcs11

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	p11 "github.com/miekg/pkcs11"
	"github.com/stretchr/testify/require"
)

func TestPoolRecoversFailedSession(t *testing.T) {
	for _, failure := range []error{p11.Error(p11.CKR_SESSION_HANDLE_INVALID), p11.Error(p11.CKR_DEVICE_REMOVED)} {
		t.Run(failure.Error(), func(t *testing.T) {
			p := newPool(nil, 1)
			t.Cleanup(p.shutdown)
			s := &sess{pool: p, handle: 1}
			p.add(s)
			got, err := p.acquire(t.Context())
			require.NoError(t, err)
			got.invalidate(failure)
			p.release(got)
			attempts := 0
			p.reopen = func(_ context.Context, current *sess) error {
				attempts++
				require.Same(t, s, current)
				if attempts == 1 {
					return errors.New("token still offline")
				}
				current.handle = 2
				return nil
			}
			_, err = p.acquire(t.Context())
			require.ErrorContains(t, err, "token still offline")
			// An unsuccessful recovery must not consume a pool slot forever.
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			got, err = p.acquire(ctx)
			require.NoError(t, err)
			require.EqualValues(t, 2, got.handle)
			require.False(t, got.broken)
			p.release(got)
			got, err = p.acquire(ctx)
			require.NoError(t, err)
			require.Equal(t, 2, attempts)
			p.release(got)
		})
	}
}

func TestPoolShutdownWaitsForRecovery(t *testing.T) {
	p := newPool(nil, 1)
	p.add(&sess{pool: p, broken: true})
	started, finish := make(chan struct{}), make(chan struct{})
	p.reopen = func(context.Context, *sess) error { close(started); <-finish; return nil }
	returned := make(chan struct{})
	go func() {
		s, err := p.acquire(context.Background())
		if err == nil {
			p.release(s)
		}
		close(returned)
	}()
	<-started
	closed := make(chan struct{})
	go func() { p.shutdown(); close(closed) }()
	select {
	case <-closed:
		t.Fatal("shutdown completed with recovery in flight")
	case <-time.After(10 * time.Millisecond):
	}
	close(finish)
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("acquire hung")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("shutdown hung")
	}
}

func TestClosedHMACSessionRejectsOperations(t *testing.T) {
	h := &hmacSession{}
	_, err := h.ChildSign([]byte{1})
	require.Error(t, err)
	_, err = h.ChildSharedX(make([]byte, 33))
	require.Error(t, err)
}

// TestGrimselSessionRecovery closes only this test's own borrowed session to
// reproduce CKR_SESSION_HANDLE_INVALID without disrupting another HSM client.
func TestGrimselSessionRecovery(t *testing.T) {
	lib, pin, label := os.Getenv("HSM_PKCS11_LIB"), os.Getenv("HSM_PIN_FILE"), os.Getenv("HSM_KEY_LABEL")
	if lib == "" || pin == "" || label == "" {
		t.Skip("set HSM_PKCS11_LIB, HSM_PIN_FILE, HSM_KEY_LABEL")
	}
	opened, err := Open(t.Context(), Config{LibraryPath: lib, PINFile: pin, KeyLabel: label, Slot: -1, SessionPoolSize: 1})
	require.NoError(t, err)
	b := opened.(*backend)
	t.Cleanup(func() { require.NoError(t, b.Close()) })
	s, err := b.pool.acquire(t.Context())
	require.NoError(t, err)
	require.NoError(t, b.module.CloseSession(s.handle))
	b.pool.release(s)
	peer := b.identity.Compressed()
	_, _, session, err := b.OpenHMAC(t.Context(), peer, []byte("2-3241645161d8 recovery"), false)
	if session != nil {
		require.NoError(t, session.Close())
	}
	require.Error(t, err, "the interrupted operation must report its failure")
	h, _, session, err := b.OpenHMAC(t.Context(), peer, []byte("2-3241645161d8 recovery"), false)
	defer zero(h)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	digest := make([]byte, 32)
	raw, err := session.ChildSign(digest)
	require.NoError(t, err)
	require.Len(t, raw, 64)
}
