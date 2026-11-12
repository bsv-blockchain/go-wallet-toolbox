package actions

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// broadcastKnownTxRepo is the knownTxRepo of a foreground broadcastTxs call. It builds on the
// background-broadcast spy for the claim and the attempts bump, and adds the status lookup and
// the BEEF build, recording every step so tests can pin the order they run in.
type broadcastKnownTxRepo struct {
	*bgSpyKnownTxRepo

	statuses map[string]wdk.ProvenTxReqStatus
	beef     *transaction.Beef
	beefErr  error
	claimErr error
}

func (r *broadcastKnownTxRepo) FindKnownTxStatuses(_ context.Context, txIDs ...string) (map[string]wdk.ProvenTxReqStatus, error) {
	found := make(map[string]wdk.ProvenTxReqStatus, len(txIDs))
	for _, txID := range txIDs {
		found[txID] = r.statuses[txID]
	}
	return found, nil
}

func (r *broadcastKnownTxRepo) GetBEEFForTxIDs(_ context.Context, _ iter.Seq[string], _ ...entity.GetBEEFOption) (*transaction.Beef, error) {
	r.rec.record("beef")
	return r.beef, r.beefErr
}

func (r *broadcastKnownTxRepo) ClaimKnownTxsForBroadcast(ctx context.Context, txIDs []string) ([]string, error) {
	r.rec.record("claim")
	if r.claimErr != nil {
		return nil, r.claimErr
	}
	return r.bgSpyKnownTxRepo.ClaimKnownTxsForBroadcast(ctx, txIDs)
}

type stubScriptsVerifier struct {
	ok  bool
	err error
}

func (s stubScriptsVerifier) VerifyScripts(_ context.Context, _ *transaction.Transaction) (bool, error) {
	return s.ok, s.err
}

// broadcastFixture is a process wired for broadcastTxs, plus the handles tests assert on.
type broadcastFixture struct {
	process  *process
	rec      *callRecorder
	knownTx  *broadcastKnownTxRepo
	services *bgStubServices
	release  *release
}

type broadcastFixtureOpts struct {
	claimReturns []string
	claimErr     error
	beefErr      error
	postErr      error
	scripts      stubScriptsVerifier
}

func newBroadcastFixture(t *testing.T, txs []*transaction.Transaction, opts broadcastFixtureOpts) *broadcastFixture {
	t.Helper()

	beef := transaction.NewBeefV2()
	statuses := make(map[string]wdk.ProvenTxReqStatus, len(txs))
	txIDs := make([]string, 0, len(txs))
	for _, tx := range txs {
		_, err := beef.MergeTransaction(tx)
		require.NoError(t, err)
		statuses[tx.TxID().String()] = wdk.ProvenTxStatusUnprocessed
		txIDs = append(txIDs, tx.TxID().String())
	}

	rec := &callRecorder{}
	knownTx := &broadcastKnownTxRepo{
		bgSpyKnownTxRepo: &bgSpyKnownTxRepo{rec: rec, claimReturns: opts.claimReturns},
		statuses:         statuses,
		beef:             beef,
		beefErr:          opts.beefErr,
		claimErr:         opts.claimErr,
	}
	services := &bgStubServices{rec: rec, result: newSuccessPostResultForAll(txIDs...), postErr: opts.postErr}

	logger := logging.NewTestLogger(t)
	rel := newRelease(logger, nil, 1)
	rel.arm("reference")

	return &broadcastFixture{
		process: &process{
			logger:          logger,
			knownTxRepo:     knownTx,
			txRepo:          bgStubTxRepo{},
			services:        services,
			scriptsVerifier: opts.scripts,
			uow:             bgUow{},
		},
		rec:      rec,
		knownTx:  knownTx,
		services: services,
		release:  rel,
	}
}

func newSuccessPostResultForAll(txIDs ...string) wdk.PostFromBeefResult {
	txIDResults := make([]wdk.PostedTxID, 0, len(txIDs))
	for _, txID := range txIDs {
		txIDResults = append(txIDResults, wdk.PostedTxID{Result: wdk.PostedTxIDResultSuccess, TxID: txID})
	}
	return wdk.PostFromBeefResult{
		&wdk.PostFromBEEFServiceResult{
			Name:             "ARC",
			PostedBEEFResult: &wdk.PostedBEEF{TxIDResults: txIDResults},
		},
	}
}

func newBroadcastTx(lockTime uint32) *transaction.Transaction {
	tx := transaction.NewTransaction()
	tx.LockTime = lockTime
	return tx
}

// newChildOf returns a transaction that spends an output of parent.
func newChildOf(parent *transaction.Transaction, lockTime uint32) *transaction.Transaction {
	child := newBroadcastTx(lockTime)
	child.Inputs = append(child.Inputs, &transaction.TransactionInput{
		SourceTXID:       parent.TxID(),
		SourceTxOutIndex: 0,
		SequenceNumber:   0xffffffff,
	})
	return child
}

func txIDsOf(txs ...*transaction.Transaction) []string {
	txIDs := make([]string, 0, len(txs))
	for _, tx := range txs {
		txIDs = append(txIDs, tx.TxID().String())
	}
	return txIDs
}

func TestBroadcastTxs_Claim(t *testing.T) {
	okScripts := stubScriptsVerifier{ok: true}

	t.Run("posts only what the claim accepted, in the order it was asked for", func(t *testing.T) {
		a, b, c := newBroadcastTx(1), newBroadcastTx(2), newBroadcastTx(3)
		ids := txIDsOf(a, b, c)
		// b is no longer claimable, and the claim answers in the database's order, not ours.
		fx := newBroadcastFixture(t, []*transaction.Transaction{a, b, c}, broadcastFixtureOpts{
			claimReturns: []string{ids[2], ids[0]},
			scripts:      okScripts,
		})

		result, err := fx.process.broadcastTxs(context.Background(), ids, false, fx.release)

		require.NoError(t, err)
		assert.Equal(t, []string{ids[0], ids[2]}, fx.services.postedTxIDs)
		require.Len(t, fx.knownTx.attemptsCalls, 1)
		assert.Equal(t, []string{ids[0], ids[2]}, fx.knownTx.attemptsCalls[0], "attempts are bumped for what is actually posted")
		require.Len(t, result.NotDelayedResults, 2)
		assert.Equal(t, ids[0], string(result.NotDelayedResults[0].TxID))
		assert.Equal(t, ids[2], string(result.NotDelayedResults[1].TxID))
		assert.False(t, fx.release.armed)
	})

	t.Run("posts nothing when nothing could be claimed", func(t *testing.T) {
		a, b := newBroadcastTx(1), newBroadcastTx(2)
		fx := newBroadcastFixture(t, []*transaction.Transaction{a, b}, broadcastFixtureOpts{
			claimReturns: []string{},
			scripts:      okScripts,
		})

		result, err := fx.process.broadcastTxs(context.Background(), txIDsOf(a, b), false, fx.release)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Empty(t, result.SendWithResults)
		assert.Empty(t, result.NotDelayedResults)
		assert.Zero(t, fx.rec.count("post"), "an unclaimed transaction must never reach the network")
		assert.Zero(t, fx.rec.count("bump"))
		assert.False(t, fx.release.armed, "claiming is the point of no return, even when it claims nothing")
	})

	t.Run("a parent is posted ahead of its child whatever order they were asked for", func(t *testing.T) {
		parent := newBroadcastTx(1)
		child := newChildOf(parent, 2)
		ids := txIDsOf(child, parent)
		fx := newBroadcastFixture(t, []*transaction.Transaction{parent, child}, broadcastFixtureOpts{scripts: okScripts})

		_, err := fx.process.broadcastTxs(context.Background(), ids, false, fx.release)

		require.NoError(t, err)
		assert.Equal(t, []string{ids[1], ids[0]}, fx.services.postedTxIDs)
	})
}

func TestBroadcastTxs_ReleaseStaysArmedUntilTheClaim(t *testing.T) {
	a := newBroadcastTx(1)
	ids := txIDsOf(a)

	t.Run("a BEEF that cannot be built keeps the inputs releasable", func(t *testing.T) {
		fx := newBroadcastFixture(t, []*transaction.Transaction{a}, broadcastFixtureOpts{
			beefErr: errors.New("boom: beef"),
			scripts: stubScriptsVerifier{ok: true},
		})

		_, err := fx.process.broadcastTxs(context.Background(), ids, false, fx.release)

		require.ErrorContains(t, err, "failed to build valid BEEF")
		assert.True(t, fx.release.armed)
		assert.Zero(t, fx.rec.count("claim"))
	})

	t.Run("a script verification failure keeps the inputs releasable", func(t *testing.T) {
		fx := newBroadcastFixture(t, []*transaction.Transaction{a}, broadcastFixtureOpts{
			scripts: stubScriptsVerifier{ok: false},
		})

		_, err := fx.process.broadcastTxs(context.Background(), ids, false, fx.release)

		require.ErrorContains(t, err, "scripts validation failed")
		assert.True(t, fx.release.armed)
		assert.Zero(t, fx.rec.count("claim"))
	})

	t.Run("a failing claim is already past the point of no return", func(t *testing.T) {
		fx := newBroadcastFixture(t, []*transaction.Transaction{a}, broadcastFixtureOpts{
			claimErr: errors.New("boom: claim"),
			scripts:  stubScriptsVerifier{ok: true},
		})

		_, err := fx.process.broadcastTxs(context.Background(), ids, false, fx.release)

		require.ErrorContains(t, err, "failed to claim txs for broadcast")
		assert.False(t, fx.release.armed)
		assert.Zero(t, fx.rec.count("post"))
	})
}

func TestBroadcastTxs_Post(t *testing.T) {
	a := newBroadcastTx(1)
	ids := txIDsOf(a)

	t.Run("claims, then counts the attempt, then posts", func(t *testing.T) {
		fx := newBroadcastFixture(t, []*transaction.Transaction{a}, broadcastFixtureOpts{scripts: stubScriptsVerifier{ok: true}})

		_, err := fx.process.broadcastTxs(context.Background(), ids, false, fx.release)

		require.NoError(t, err)
		assert.Equal(t, []string{"beef", "claim", "bump", "post"}, fx.rec.snapshot())
	})

	t.Run("a failed post still leaves the attempt counted and the inputs spent", func(t *testing.T) {
		fx := newBroadcastFixture(t, []*transaction.Transaction{a}, broadcastFixtureOpts{
			postErr: errors.New("boom: post"),
			scripts: stubScriptsVerifier{ok: true},
		})

		_, err := fx.process.broadcastTxs(context.Background(), ids, false, fx.release)

		require.ErrorContains(t, err, "failed to post BEEF")
		assert.Equal(t, 1, fx.rec.count("bump"))
		assert.False(t, fx.release.armed, "the transaction may already be on the network")
	})
}
