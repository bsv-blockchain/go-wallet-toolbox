package actions

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Everything after the funding transaction commits happens on an action that already holds
// its reserved inputs, while the caller never receives the reference - so nothing else can
// ever release them. These tests pin the compensation contract used by both post-commit
// failure paths in create.go (resultInputs and mergeAllocatedUTXOs).

type stubAborter struct {
	err error

	calls []stubAbortCall
}

type stubAbortCall struct {
	userID    int
	reference string
	ctxErr    error
}

func (s *stubAborter) AbortAction(ctx context.Context, userID int, args *wdk.AbortActionArgs) (*wdk.AbortActionResult, error) {
	s.calls = append(s.calls, stubAbortCall{
		userID:    userID,
		reference: string(args.Reference),
		ctxErr:    ctx.Err(),
	})
	if s.err != nil {
		return nil, s.err
	}
	return &wdk.AbortActionResult{Aborted: true}, nil
}

func newCreateWithAborter(aborter actionAborter) *create {
	return &create{
		logger:  slog.New(slog.DiscardHandler),
		aborter: aborter,
	}
}

func TestCreate_FailAfterTxSaved_ReleasesReservedInputs(t *testing.T) {
	// given:
	aborter := &stubAborter{}
	c := newCreateWithAborter(aborter)
	cause := errors.New("failed to build result inputs")

	// when:
	err := c.failAfterTxSaved(t.Context(), 42, "ref-1", cause)

	// then: the original error is returned unchanged
	require.ErrorIs(t, err, cause)

	// and: the saved action is released
	require.Len(t, aborter.calls, 1)
	assert.Equal(t, 42, aborter.calls[0].userID)
	assert.Equal(t, "ref-1", aborter.calls[0].reference)
}

func TestCreate_FailAfterTxSaved_RunsOnCanceledContext(t *testing.T) {
	// given:
	aborter := &stubAborter{}
	c := newCreateWithAborter(aborter)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// when:
	err := c.failAfterTxSaved(ctx, 1, "ref-canceled", errors.New("boom"))

	// then: the release is detached from the canceled request context
	require.Error(t, err)
	require.Len(t, aborter.calls, 1)
	assert.NoError(t, aborter.calls[0].ctxErr, "the abort must not inherit the canceled context")
}

func TestCreate_FailAfterTxSaved_AbortFailureDoesNotMaskCause(t *testing.T) {
	// given:
	aborter := &stubAborter{err: errors.New("storage is down")}
	c := newCreateWithAborter(aborter)
	cause := errors.New("failed to create BEEF with allocated UTXOs")

	// when:
	err := c.failAfterTxSaved(t.Context(), 1, "ref-2", cause)

	// then:
	require.ErrorIs(t, err, cause)
	assert.NotContains(t, err.Error(), "storage is down")
	assert.Len(t, aborter.calls, 1)
}

func TestCreate_FailAfterTxSaved_NoReferenceOrAborter(t *testing.T) {
	tests := map[string]struct {
		aborter   *stubAborter
		reference string
	}{
		"missing reference": {aborter: &stubAborter{}, reference: ""},
		"no aborter wired":  {aborter: nil, reference: "ref-3"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			var aborter actionAborter
			if test.aborter != nil {
				aborter = test.aborter
			}
			c := newCreateWithAborter(aborter)
			cause := errors.New("boom")

			// when:
			err := c.failAfterTxSaved(t.Context(), 1, test.reference, cause)

			// then:
			require.ErrorIs(t, err, cause)
			if test.aborter != nil {
				assert.Empty(t, test.aborter.calls)
			}
		})
	}
}
