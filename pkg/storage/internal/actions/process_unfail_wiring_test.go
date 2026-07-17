package actions

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// --- minimal stubs: embed the interface, override only what the invalid path touches ---

type stubOutputRepo struct {
	OutputRepo // embed interface; unimplemented methods panic if unexpectedly called

	recreateErr     error
	notSpendableErr error

	recreateCalls     []uint
	notSpendableCalls []uint
}

func (s *stubOutputRepo) RecreateSpentOutputs(_ context.Context, transactionID uint) error {
	s.recreateCalls = append(s.recreateCalls, transactionID)
	return s.recreateErr
}

func (s *stubOutputRepo) MarkCreatedOutputsAsNotSpendable(_ context.Context, transactionID uint) error {
	s.notSpendableCalls = append(s.notSpendableCalls, transactionID)
	return s.notSpendableErr
}

type stubTransactionsRepo struct {
	TransactionsRepo // embed interface; unimplemented methods panic if unexpectedly called

	transactionIDs []uint
}

func (stubTransactionsRepo) UpdateTransactionStatusByTxID(_ context.Context, _ string, _ wdk.TxStatus, _ ...wdk.TxStatus) error {
	return nil
}

func (s stubTransactionsRepo) FindTransactionIDsByTxID(_ context.Context, _ string) ([]uint, error) {
	return s.transactionIDs, nil
}

type stubKnownTxRepo struct {
	KnownTxRepo // embed interface; unimplemented methods panic if unexpectedly called
}

func (stubKnownTxRepo) UpdateKnownTxStatus(_ context.Context, _ string, _ wdk.ProvenTxReqStatus, _ []wdk.ProvenTxReqStatus, _ []history.Builder) error {
	return nil
}

type stubProviders struct {
	Providers // embed interface; unimplemented accessors panic if unexpectedly called

	txRepo      TransactionsRepo
	outputRepo  OutputRepo
	knownTxRepo KnownTxRepo
}

func (s stubProviders) TransactionsRepo() TransactionsRepo { return s.txRepo }
func (s stubProviders) OutputRepo() OutputRepo             { return s.outputRepo }
func (s stubProviders) KnownTxRepo() KnownTxRepo           { return s.knownTxRepo }

// stubUnitOfWork just invokes fn with the stub providers, mirroring the real
// UnitOfWork's contract of running the callback within (what would be) a transaction.
type stubUnitOfWork struct {
	providers Providers
}

func (u stubUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context, p Providers) error) error {
	return fn(ctx, u.providers)
}

func TestUnfailInvalidPathPropagates(t *testing.T) {
	// given: RecreateSpentOutputs fails for the transaction being restored
	injectedErr := errors.New("boom: recreate spent outputs failed")
	outputRepo := &stubOutputRepo{recreateErr: injectedErr}

	uow := stubUnitOfWork{
		providers: stubProviders{
			txRepo:      stubTransactionsRepo{transactionIDs: []uint{42}},
			outputRepo:  outputRepo,
			knownTxRepo: stubKnownTxRepo{},
		},
	}

	p := &process{
		logger: logging.NewTestLogger(t),
		uow:    uow,
	}

	// when: the invalid path (KnownTx -> invalid, cascade to Transactions -> failed,
	// restore spent outputs) runs inside the UnitOfWork
	err := p.markAsInvalid(context.Background(), "deadbeef")

	// then: the restore error must propagate out of the UoW callback so the whole
	// cascade rolls back, instead of being swallowed (log-and-continue) while the
	// KnownTx/Transactions status changes still commit.
	require.Error(t, err)
	assert.ErrorIs(t, err, injectedErr)

	// and: MarkCreatedOutputsAsNotSpendable must not run once RecreateSpentOutputs
	// has already failed for that transaction id (the loop returns immediately).
	assert.Empty(t, outputRepo.notSpendableCalls)
}
