package actions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const syncTestHeight = uint(850_000)

var errSyncBoom = errors.New("boom")

// syncKnownTxRepo is the knownTxRepo of the status sync. Every step records itself so tests can
// pin the order the steps run in.
type syncKnownTxRepo struct {
	KnownTxRepo // embed interface; unimplemented methods panic if unexpectedly called

	rec *callRecorder

	// candidatePages returns the page of candidates for the n-th (0-based) find call.
	candidatePages func(call int) []*entity.KnownTxForStatusSync
	findCalls      int
	findStatuses   [][]wdk.ProvenTxReqStatus
	findErr        error

	mined       []*entity.KnownTxAsMined
	minedErrFor map[string]error

	attemptBumps [][]string
	attemptsErr  error

	timeoutResult []models.KnownTx
	timeoutArgs   []syncTimeoutArgs
	timeoutErr    error

	// reviewPages returns the page of failed txs for the n-th (0-based) review call.
	reviewPages  func(call int) []*entity.KnownTxForStatusSync
	reviewCalls  int
	reviewLimits []int
	reviewStatus [][]wdk.ProvenTxReqStatus
	reviewErr    error
}

type syncTimeoutArgs struct {
	attempts, maxRebroadcastAttempts uint64
	statuses                         []wdk.ProvenTxReqStatus
}

func (r *syncKnownTxRepo) FindKnownTxIDsReadyForStatusSync(_ context.Context, statuses []wdk.ProvenTxReqStatus, _ time.Duration, _ ...queryopts.Options) ([]*entity.KnownTxForStatusSync, error) {
	r.rec.record("find")
	r.findStatuses = append(r.findStatuses, statuses)
	call := r.findCalls
	r.findCalls++
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.candidatePages == nil {
		return nil, nil
	}
	return r.candidatePages(call), nil
}

func (r *syncKnownTxRepo) UpdateKnownTxAsMined(_ context.Context, mined *entity.KnownTxAsMined) error {
	r.rec.record("mined:" + mined.TxID)
	if err := r.minedErrFor[mined.TxID]; err != nil {
		return err
	}
	r.mined = append(r.mined, mined)
	return nil
}

func (r *syncKnownTxRepo) IncreaseKnownTxAttemptsForTxIDs(_ context.Context, txIDs []string) error {
	r.rec.record("attempts")
	r.attemptBumps = append(r.attemptBumps, txIDs)
	return r.attemptsErr
}

func (r *syncKnownTxRepo) ApplyProofTimeouts(_ context.Context, attempts, maxRebroadcastAttempts uint64, statuses []wdk.ProvenTxReqStatus) ([]models.KnownTx, error) {
	r.rec.record("timeouts")
	r.timeoutArgs = append(r.timeoutArgs, syncTimeoutArgs{attempts, maxRebroadcastAttempts, statuses})
	return r.timeoutResult, r.timeoutErr
}

func (r *syncKnownTxRepo) FindKnownTxIDsByStatusesNeedingFailureReview(_ context.Context, statuses []wdk.ProvenTxReqStatus, limit int) ([]*entity.KnownTxForStatusSync, error) {
	r.rec.record("review")
	r.reviewStatus = append(r.reviewStatus, statuses)
	r.reviewLimits = append(r.reviewLimits, limit)
	call := r.reviewCalls
	r.reviewCalls++
	if r.reviewErr != nil {
		return nil, r.reviewErr
	}
	if r.reviewPages == nil {
		return nil, nil
	}
	return r.reviewPages(call), nil
}

// syncTxRepo is the transactionRepo of the status sync, also used inside the unit of work of the
// failure review.
type syncTxRepo struct {
	TransactionsRepo // embed interface; unimplemented methods panic if unexpectedly called

	references map[string]string
	labels     map[string][]string
	ids        map[string][]uint

	referenceLookups [][]string
	referencesErr    error
	labelsErr        error
	idsErrFor        map[string]error

	failedTransactionIDs []uint
	statusUpdateErr      error
}

func (r *syncTxRepo) FindReferencesByTxIDs(_ context.Context, txIDs []string) (map[string]string, error) {
	r.referenceLookups = append(r.referenceLookups, txIDs)
	return r.references, r.referencesErr
}

func (r *syncTxRepo) GetLabelsForTxIDs(_ context.Context, _ []string) (map[string][]string, error) {
	return r.labels, r.labelsErr
}

func (r *syncTxRepo) FindTransactionIDsByTxID(_ context.Context, txID string) ([]uint, error) {
	if err := r.idsErrFor[txID]; err != nil {
		return nil, err
	}
	return r.ids[txID], nil
}

func (r *syncTxRepo) UpdateTransactionStatusByID(_ context.Context, transactionID uint, status wdk.TxStatus, _ ...wdk.TxStatus) error {
	if r.statusUpdateErr != nil {
		return r.statusUpdateErr
	}
	if status == wdk.TxStatusFailed {
		r.failedTransactionIDs = append(r.failedTransactionIDs, transactionID)
	}
	return nil
}

// syncOutputRepo records what the failure review does to outputs. restored must stay empty: the
// spent inputs of a failed tx are never made spendable again.
type syncOutputRepo struct {
	OutputRepo // embed interface; unimplemented methods panic if unexpectedly called

	notSpendable []uint
	restored     []uint
	markErr      error
}

func (r *syncOutputRepo) MarkCreatedOutputsAsNotSpendable(_ context.Context, transactionID uint) error {
	if r.markErr != nil {
		return r.markErr
	}
	r.notSpendable = append(r.notSpendable, transactionID)
	return nil
}

func (r *syncOutputRepo) RecreateSpentOutputs(_ context.Context, spendingTransactionID uint) error {
	r.restored = append(r.restored, spendingTransactionID)
	return nil
}

type syncKeyValueRepo struct {
	data   map[string][]byte
	getErr error
}

func (r *syncKeyValueRepo) Get(_ context.Context, key string) ([]byte, bool, error) {
	if r.getErr != nil {
		return nil, false, r.getErr
	}
	value, ok := r.data[key]
	return value, ok, nil
}

func (r *syncKeyValueRepo) Set(_ context.Context, key string, value []byte) error {
	r.data[key] = value
	return nil
}

type syncMerkleReply struct {
	result *wdk.MerklePathResult
	err    error
}

type syncServices struct {
	wdk.Services // embed interface; unimplemented methods panic if unexpectedly called

	rec *callRecorder

	details     []wdk.TxStatusDetail
	statusErr   error
	statusCalls [][]string

	merkle       map[string]syncMerkleReply
	onMerklePath func(txID string)
}

func (s *syncServices) GetStatusForTxIDs(_ context.Context, txIDs []string) (*wdk.GetStatusForTxIDsResult, error) {
	s.rec.record("depth")
	s.statusCalls = append(s.statusCalls, txIDs)
	if s.statusErr != nil {
		return nil, s.statusErr
	}
	return &wdk.GetStatusForTxIDsResult{Status: wdk.GetStatusSuccess, Results: s.details}, nil
}

func (s *syncServices) MerklePath(_ context.Context, txID string) (*wdk.MerklePathResult, error) {
	s.rec.record("merkle:" + txID)
	if s.onMerklePath != nil {
		s.onMerklePath(txID)
	}
	reply, ok := s.merkle[txID]
	if !ok {
		return nil, wdk.ErrNotFoundError
	}
	return reply.result, reply.err
}

type syncProviders struct {
	Providers // embed interface; unimplemented methods panic if unexpectedly called

	txRepo     *syncTxRepo
	outputRepo *syncOutputRepo
}

func (p syncProviders) TransactionsRepo() TransactionsRepo { return p.txRepo }
func (p syncProviders) OutputRepo() OutputRepo             { return p.outputRepo }

type syncUow struct {
	providers syncProviders
	err       error
}

func (u syncUow) Do(ctx context.Context, fn func(ctx context.Context, p Providers) error) error {
	if u.err != nil {
		return u.err
	}
	return fn(ctx, u.providers)
}

type syncFixture struct {
	sync     *synchronizeTxStatuses
	rec      *callRecorder
	cfg      defs.SynchronizeTxStatuses
	knownTx  *syncKnownTxRepo
	txRepo   *syncTxRepo
	outputs  *syncOutputRepo
	services *syncServices
	kv       *syncKeyValueRepo
	uow      *syncUow
}

func newSyncFixture(t *testing.T) *syncFixture {
	t.Helper()

	rec := &callRecorder{}
	cfg := defs.SynchronizeTxStatuses{
		MaxAttempts:            7,
		MaxRebroadcastAttempts: 3,
		CheckNoSendPeriodHours: 24,
		BlocksDelay:            2,
	}
	knownTx := &syncKnownTxRepo{rec: rec, minedErrFor: map[string]error{}}
	txRepo := &syncTxRepo{
		references: map[string]string{},
		labels:     map[string][]string{},
		ids:        map[string][]uint{},
		idsErrFor:  map[string]error{},
	}
	outputs := &syncOutputRepo{}
	services := &syncServices{rec: rec, merkle: map[string]syncMerkleReply{}}
	// A recent no-send check keeps the statuses to the plain statusesReadyToSync.
	kv := &syncKeyValueRepo{data: map[string][]byte{noSendLastCheck: []byte(time.Now().Format(time.RFC3339))}}
	uow := &syncUow{providers: syncProviders{txRepo: txRepo, outputRepo: outputs}}

	return &syncFixture{
		sync: newSynchronizeTxStatuses(
			logging.NewTestLogger(t), cfg, 0, services, knownTx, kv, txRepo, outputs, uow,
		),
		rec:      rec,
		cfg:      cfg,
		knownTx:  knownTx,
		txRepo:   txRepo,
		outputs:  outputs,
		services: services,
		kv:       kv,
		uow:      uow,
	}
}

// givenCandidates makes the sync find these txs, all on the first page.
func (f *syncFixture) givenCandidates(txIDs ...string) {
	txs := make([]*entity.KnownTxForStatusSync, 0, len(txIDs))
	for _, txID := range txIDs {
		txs = append(txs, syncCandidate(txID))
	}
	f.knownTx.candidatePages = func(call int) []*entity.KnownTxForStatusSync {
		if call == 0 {
			return txs
		}
		return nil
	}
}

// givenMined makes the network report the tx deep enough and serve its merkle path, and gives
// the tx two transaction rows, a reference and a label.
func (f *syncFixture) givenMined(txID string, height uint32) {
	depth := int(f.cfg.BlocksDelay) + 1
	f.services.details = append(f.services.details, wdk.TxStatusDetail{
		TxID: txID, Depth: &depth, Status: wdk.ResultStatusForTxIDMined.String(),
	})
	f.services.merkle[txID] = syncMerkleReply{result: syncMinedResult(txID, height)}
	f.txRepo.ids[txID] = []uint{7, 8}
	f.txRepo.references[txID] = "ref-" + txID
	f.txRepo.labels[txID] = []string{"label-" + txID}
}

// givenConfirmedButUnprovable makes the network report the tx deep enough, with the given
// answer to the merkle path query.
func (f *syncFixture) givenConfirmedButUnprovable(txID string, reply syncMerkleReply) {
	depth := int(f.cfg.BlocksDelay)
	f.services.details = append(f.services.details, wdk.TxStatusDetail{
		TxID: txID, Depth: &depth, Status: wdk.ResultStatusForTxIDMined.String(),
	})
	f.services.merkle[txID] = reply
}

// givenAbsent makes the network answer NOT_FOUND for the tx.
func (f *syncFixture) givenAbsent(txID string) {
	f.services.details = append(f.services.details, wdk.TxStatusDetail{
		TxID: txID, Status: wdk.ResultStatusForTxIDNotFound.String(),
	})
	f.txRepo.references[txID] = "ref-" + txID
	f.txRepo.labels[txID] = []string{"label-" + txID}
}

func (f *syncFixture) run(ctx context.Context) ([]wdk.TxSynchronizedStatus, error) {
	return f.sync.doSynchronizeTxStatuses(ctx, syncTestHeight)
}

func syncCandidate(txID string) *entity.KnownTxForStatusSync {
	return &entity.KnownTxForStatusSync{TxID: txID, Status: wdk.ProvenTxStatusUnmined, Attempts: 1}
}

func syncMinedResult(txID string, height uint32) *wdk.MerklePathResult {
	isTxID := true
	leaf := chainhash.HashH([]byte(txID))
	return &wdk.MerklePathResult{
		Name: "test",
		MerklePath: transaction.NewMerklePath(height, [][]*transaction.PathElement{
			{{Offset: 0, Txid: &isTxID, Hash: &leaf}},
		}),
		BlockHeader: &wdk.MerklePathBlockHeader{
			Height:     height,
			Hash:       "hash-" + txID,
			MerkleRoot: "root-" + txID,
		},
	}
}

func syncTxIDs(statuses []wdk.TxSynchronizedStatus) []string {
	txIDs := make([]string, 0, len(statuses))
	for _, status := range statuses {
		txIDs = append(txIDs, status.TxID)
	}
	return txIDs
}
