package pending_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/pending"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestLocalPendingSignActionsCache_SetGetDelete_Success(t *testing.T) {
	// given:
	cache := pending.NewSignActionLocalRepository(slog.Default(), -1)

	// and:
	ref := "ref1"
	action := &pending.SignAction{}

	// when:
	err := cache.Save(ref, action)

	// then:
	require.NoError(t, err)

	// when:
	got, err := cache.Get(ref)

	// then:
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, *action, *got)

	// when:
	err = cache.Delete(ref)

	// then:
	require.NoError(t, err)
}

func TestLocalPendingSignActionsCache_Get_Error_NotFound(t *testing.T) {
	tests := map[string]struct {
		ref string
	}{
		"missing_reference": {ref: "unknown-ref"},
		"empty_reference":   {ref: ""},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			cache := pending.NewSignActionLocalRepository(slog.Default(), -1)

			// when:
			_, err := cache.Get(test.ref)

			// then:
			require.Error(t, err)
			assert.ErrorIs(t, err, wdk.ErrNotFoundError)
		})
	}
}

func TestLocalPendingSignActionsCache_TTL_Cleanup_KeepsFresh_Success(t *testing.T) {
	t.Parallel()

	// given:
	ttl := 10 * time.Millisecond
	cache := pending.NewSignActionLocalRepository(slog.Default(), ttl)

	// and:
	oldRef := "old"
	newRef := "new"
	action := &pending.SignAction{}

	// when:
	err := cache.Save(oldRef, action)

	// then:
	require.NoError(t, err)

	// when:
	time.Sleep(ttl + time.Second + 50*time.Millisecond)
	err = cache.Save(newRef, action)

	// then:
	require.NoError(t, err)

	// when:
	got, err := cache.Get(newRef)

	// then:
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, *action, *got)

	// when:
	got, err = cache.Get(oldRef)

	// then:
	require.ErrorIs(t, err, wdk.ErrNotFoundError)
	require.Nil(t, got)
}
