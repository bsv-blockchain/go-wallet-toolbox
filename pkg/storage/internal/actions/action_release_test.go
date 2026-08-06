package actions

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// The release is the single compensation used by createAction and processAction. Its whole
// contract is: fire on error while armed, never fire once disarmed (the point of no return),
// and never let its own problems reach the caller.

type stubAborter struct {
	err error

	byReference []string
	byTxID      []string
	userIDs     []int
	ctxErrs     []error
}

func (s *stubAborter) AbortAction(ctx context.Context, userID int, args *wdk.AbortActionArgs) (*wdk.AbortActionResult, error) {
	s.byReference = append(s.byReference, string(args.Reference))
	s.userIDs = append(s.userIDs, userID)
	s.ctxErrs = append(s.ctxErrs, ctx.Err())
	if s.err != nil {
		return nil, s.err
	}
	return &wdk.AbortActionResult{Aborted: true}, nil
}

func (s *stubAborter) AbortUnbroadcastTx(ctx context.Context, userID int, txID string) error {
	s.byTxID = append(s.byTxID, txID)
	s.userIDs = append(s.userIDs, userID)
	s.ctxErrs = append(s.ctxErrs, ctx.Err())
	return s.err
}

func (s *stubAborter) calls() int {
	return len(s.byReference) + len(s.byTxID)
}

func newTestRelease(aborter actionAborter) *release {
	return newRelease(slog.New(slog.DiscardHandler), aborter, 42)
}

func TestRelease_ArmedByReference_ReleasesOnError(t *testing.T) {
	// given:
	aborter := &stubAborter{}
	rel := newTestRelease(aborter)
	rel.armByReference("ref-1")

	// when:
	rel.onError(t.Context(), errors.New("boom"))

	// then:
	assert.Equal(t, []string{"ref-1"}, aborter.byReference)
	assert.Equal(t, []int{42}, aborter.userIDs)
	assert.Empty(t, aborter.byTxID)
}

func TestRelease_ArmedByTxID_ParksAndReleases(t *testing.T) {
	// given: the action already carries a txid, so the KnownTx-parking path must be used
	aborter := &stubAborter{}
	rel := newTestRelease(aborter)
	rel.armByReference("ref-1")
	rel.armByTxID("tx-1")

	// when:
	rel.onError(t.Context(), errors.New("boom"))

	// then:
	assert.Equal(t, []string{"tx-1"}, aborter.byTxID)
	assert.Empty(t, aborter.byReference)
}

func TestRelease_DoesNotFireWhenNotApplicable(t *testing.T) {
	tests := map[string]func(rel *release){
		"never armed":         func(_ *release) {},
		"armed then disarmed": func(rel *release) { rel.armByReference("ref"); rel.disarm() },
		"empty reference":     func(rel *release) { rel.armByReference("") },
		"empty txID":          func(rel *release) { rel.armByTxID("") },
	}

	for name, setup := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			aborter := &stubAborter{}
			rel := newTestRelease(aborter)
			setup(rel)

			// when:
			rel.onError(t.Context(), errors.New("boom"))

			// then:
			assert.Zero(t, aborter.calls())
		})
	}
}

func TestRelease_DoesNotFireOnSuccess(t *testing.T) {
	// given:
	aborter := &stubAborter{}
	rel := newTestRelease(aborter)
	rel.armByReference("ref-1")

	// when: the deferred call of a successful operation
	rel.onError(t.Context(), nil)

	// then:
	assert.Zero(t, aborter.calls())
}

func TestRelease_FiresOnlyOnce(t *testing.T) {
	// given:
	aborter := &stubAborter{}
	rel := newTestRelease(aborter)
	rel.armByReference("ref-1")

	// when:
	rel.onError(t.Context(), errors.New("boom"))
	rel.onError(t.Context(), errors.New("boom again"))

	// then:
	assert.Equal(t, 1, aborter.calls())
}

func TestRelease_RunsOnCanceledContext(t *testing.T) {
	// given:
	aborter := &stubAborter{}
	rel := newTestRelease(aborter)
	rel.armByReference("ref-1")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// when:
	rel.onError(ctx, errors.New("request canceled"))

	// then: the release is detached from the canceled request context
	require.Len(t, aborter.ctxErrs, 1)
	assert.NoError(t, aborter.ctxErrs[0])
}

func TestRelease_AbortFailureIsSwallowed(t *testing.T) {
	// given:
	aborter := &stubAborter{err: errors.New("storage is down")}
	rel := newTestRelease(aborter)
	rel.armByReference("ref-1")

	// when / then: no panic, no propagation - the caller keeps its original error
	rel.onError(t.Context(), errors.New("boom"))
	assert.Equal(t, 1, aborter.calls())
}

func TestRelease_NilIsPermanentlyDisarmed(t *testing.T) {
	// given: the send_waiting sweep owns no action and passes nil
	var rel *release

	// when / then: every transition is a safe no-op
	rel.armByReference("ref")
	rel.armByTxID("tx")
	rel.disarm()
	rel.onError(t.Context(), errors.New("boom"))
}

func TestProcess_NewRelease_OnlyArmsForOwnTransaction(t *testing.T) {
	reference := "ref-1"

	tests := map[string]struct {
		args        *wdk.ProcessActionArgs
		expectArmed bool
	}{
		"new transaction of this call": {
			args:        &wdk.ProcessActionArgs{IsNewTx: true, Reference: &reference},
			expectArmed: true,
		},
		"sendWith-only call": {
			args:        &wdk.ProcessActionArgs{IsNewTx: false, IsSendWith: true},
			expectArmed: false,
		},
		"new transaction without reference": {
			args:        &wdk.ProcessActionArgs{IsNewTx: true},
			expectArmed: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			aborter := &stubAborter{}
			p := &process{logger: slog.New(slog.DiscardHandler), aborter: aborter}

			// when:
			rel := p.newRelease(1, test.args)
			rel.onError(t.Context(), errors.New("boom"))

			// then:
			assert.Equal(t, test.expectArmed, aborter.calls() == 1)
		})
	}
}

// A pre-broadcast release must only touch rows the caller owns, and must back off entirely
// when any other row keeps the shared transaction alive.
func TestSplitTxsForPreBroadcastAbort(t *testing.T) {
	const (
		alice = 1
		bob   = 2
	)

	tests := map[string]struct {
		transactions      []*pkgentity.Transaction
		userScope         *int
		expectedToAbort   []uint
		expectOthersAlive bool
	}{
		"single owner in an abortable status": {
			transactions: []*pkgentity.Transaction{
				{ID: 10, UserID: alice, Status: wdk.TxStatusUnprocessed},
			},
			userScope:       ptr(alice),
			expectedToAbort: []uint{10},
		},
		"row of another user is neither aborted nor ignored": {
			transactions: []*pkgentity.Transaction{
				{ID: 10, UserID: alice, Status: wdk.TxStatusUnprocessed},
				{ID: 11, UserID: bob, Status: wdk.TxStatusUnprocessed},
			},
			userScope:         ptr(alice),
			expectedToAbort:   []uint{10},
			expectOthersAlive: true,
		},
		"already aborted rows of other users do not keep the tx alive": {
			transactions: []*pkgentity.Transaction{
				{ID: 10, UserID: alice, Status: wdk.TxStatusUnprocessed},
				{ID: 11, UserID: bob, Status: wdk.TxStatusAborted},
				{ID: 12, UserID: bob, Status: wdk.TxStatusFailed},
			},
			userScope:       ptr(alice),
			expectedToAbort: []uint{10},
		},
		"sending is not abortable and keeps the tx alive": {
			transactions: []*pkgentity.Transaction{
				{ID: 10, UserID: alice, Status: wdk.TxStatusSending},
			},
			userScope:         ptr(alice),
			expectOthersAlive: true,
		},
		"broadcast rows are never abortable": {
			transactions: []*pkgentity.Transaction{
				{ID: 10, UserID: alice, Status: wdk.TxStatusUnproven},
			},
			userScope:         ptr(alice),
			expectOthersAlive: true,
		},
		"storage-wide scope covers every owner": {
			transactions: []*pkgentity.Transaction{
				{ID: 10, UserID: alice, Status: wdk.TxStatusUnprocessed},
				{ID: 11, UserID: bob, Status: wdk.TxStatusUnprocessed},
			},
			userScope:       nil,
			expectedToAbort: []uint{10, 11},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			a := &abortAction{}

			// when:
			toAbort, othersStillActive := a.splitTxsForPreBroadcastAbort(test.transactions, test.userScope)

			// then:
			assert.Equal(t, test.expectedToAbort, toAbort)
			assert.Equal(t, test.expectOthersAlive, othersStillActive)
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}
