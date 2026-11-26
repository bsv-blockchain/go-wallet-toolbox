package actions

import (
	"context"
	"errors"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/require"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const internalizeTestUserID = 7

var errInternalizeBoom = errors.New("boom")

// internalizeBeefVerifier answers VerifyBeef with a fixed outcome.
type internalizeBeefVerifier struct {
	ok  bool
	err error
}

func (v *internalizeBeefVerifier) VerifyBeef(context.Context, *transaction.Beef, bool) (bool, error) {
	return v.ok, v.err
}

// internalizeScriptsVerifier answers VerifyScripts with a fixed outcome and records which txs it saw.
type internalizeScriptsVerifier struct {
	ok      bool
	err     error
	checked []string
}

func (v *internalizeScriptsVerifier) VerifyScripts(_ context.Context, tx *transaction.Transaction) (bool, error) {
	v.checked = append(v.checked, tx.TxID().String())
	return v.ok, v.err
}

// internalizeKnownTxRepo serves both the internalize action and the process it broadcasts through.
type internalizeKnownTxRepo struct {
	KnownTxRepo // embed interface; unimplemented methods panic if unexpectedly called

	statuses map[string]wdk.ProvenTxReqStatus
	upserted []*entity.UpsertKnownTx
	claimed  [][]string
	claimErr error
}

func (r *internalizeKnownTxRepo) FindKnownTxStatuses(_ context.Context, txIDs ...string) (map[string]wdk.ProvenTxReqStatus, error) {
	found := map[string]wdk.ProvenTxReqStatus{}
	for _, txID := range txIDs {
		if status, ok := r.statuses[txID]; ok {
			found[txID] = status
		}
	}
	return found, nil
}

func (r *internalizeKnownTxRepo) UpsertKnownTx(_ context.Context, req *entity.UpsertKnownTx, _ history.Builder) error {
	r.upserted = append(r.upserted, req)
	return nil
}

func (r *internalizeKnownTxRepo) ClaimKnownTxsForBroadcast(_ context.Context, txIDs []string) ([]string, error) {
	r.claimed = append(r.claimed, txIDs)
	return nil, r.claimErr
}

type internalizeLabelsCall struct {
	userID        int
	transactionID uint
	labels        []string
}

type internalizeTxRepo struct {
	TransactionsRepo // embed interface; unimplemented methods panic if unexpectedly called

	stored  *pkgentity.Transaction
	created []*entity.NewTx
	labels  []internalizeLabelsCall
}

func (r *internalizeTxRepo) FindTransactionByUserIDAndTxID(context.Context, int, string) (*pkgentity.Transaction, error) {
	return r.stored, nil
}

func (r *internalizeTxRepo) CreateTransaction(_ context.Context, tx *entity.NewTx) error {
	r.created = append(r.created, tx)
	return nil
}

func (r *internalizeTxRepo) AddLabels(_ context.Context, userID int, transactionID uint, labels ...string) error {
	r.labels = append(r.labels, internalizeLabelsCall{userID: userID, transactionID: transactionID, labels: labels})
	return nil
}

type internalizeBasketRepo struct {
	BasketRepo // embed interface; unimplemented methods panic if unexpectedly called

	changeBasket *pkgentity.OutputBasket
	ensured      []string
}

func (r *internalizeBasketRepo) FindBasketByName(context.Context, int, string) (*pkgentity.OutputBasket, error) {
	return r.changeBasket, nil
}

func (r *internalizeBasketRepo) FindOrCreateBasket(_ context.Context, _ int, name string) error {
	r.ensured = append(r.ensured, name)
	return nil
}

type internalizeOutputRepo struct {
	OutputRepo // embed interface; unimplemented methods panic if unexpectedly called

	saved [][]*pkgentity.Output
}

// FindOutput reports every output as new, so a merge creates rather than updates outputs.
func (r *internalizeOutputRepo) FindOutput(context.Context, int, wdk.OutPoint) (*pkgentity.Output, error) {
	return nil, nil
}

func (r *internalizeOutputRepo) SaveOutputs(_ context.Context, outputs []*pkgentity.Output) error {
	r.saved = append(r.saved, outputs)
	return nil
}

type internalizeProviders struct {
	Providers // embed interface; unimplemented methods panic if unexpectedly called

	txRepo      *internalizeTxRepo
	knownTxRepo *internalizeKnownTxRepo
	basketRepo  *internalizeBasketRepo
	outputRepo  *internalizeOutputRepo
}

func (p internalizeProviders) TransactionsRepo() TransactionsRepo { return p.txRepo }
func (p internalizeProviders) KnownTxRepo() KnownTxRepo           { return p.knownTxRepo }
func (p internalizeProviders) BasketRepo() BasketRepo             { return p.basketRepo }
func (p internalizeProviders) OutputRepo() OutputRepo             { return p.outputRepo }

type internalizeUow struct {
	providers internalizeProviders
	calls     int
}

func (u *internalizeUow) Do(ctx context.Context, fn func(ctx context.Context, p Providers) error) error {
	u.calls++
	return fn(ctx, u.providers)
}

type internalizeFixture struct {
	action          *internalize
	beefVerifier    *internalizeBeefVerifier
	scriptsVerifier *internalizeScriptsVerifier
	knownTx         *internalizeKnownTxRepo
	txRepo          *internalizeTxRepo
	baskets         *internalizeBasketRepo
	outputs         *internalizeOutputRepo
	uow             *internalizeUow
}

// newInternalizeFixture wires an internalize action whose verifiers accept everything, whose
// storage holds no matching transaction and whose broadcast claim fails (a hard broadcast error).
func newInternalizeFixture(t *testing.T) *internalizeFixture {
	t.Helper()

	logger := logging.NewTestLogger(t)
	beefVerifier := &internalizeBeefVerifier{ok: true}
	scriptsVerifier := &internalizeScriptsVerifier{ok: true}
	knownTx := &internalizeKnownTxRepo{statuses: map[string]wdk.ProvenTxReqStatus{}, claimErr: errInternalizeBoom}
	txRepo := &internalizeTxRepo{}
	baskets := &internalizeBasketRepo{
		changeBasket: &pkgentity.OutputBasket{Name: wdk.BasketNameForChange, UserID: internalizeTestUserID},
	}
	outputs := &internalizeOutputRepo{}
	uow := &internalizeUow{providers: internalizeProviders{
		txRepo: txRepo, knownTxRepo: knownTx, basketRepo: baskets, outputRepo: outputs,
	}}
	// Only the claim is reachable on this process: it fails before anything else is touched.
	broadcaster := &process{logger: logger, knownTxRepo: knownTx}

	return &internalizeFixture{
		action: newInternalizeAction(
			logger, txRepo, baskets, knownTx, outputs, uow,
			randomizer.NewTestRandomizer(), beefVerifier, scriptsVerifier, nil, broadcaster,
		),
		beefVerifier:    beefVerifier,
		scriptsVerifier: scriptsVerifier,
		knownTx:         knownTx,
		txRepo:          txRepo,
		baskets:         baskets,
		outputs:         outputs,
		uow:             uow,
	}
}

// givenStoredTx makes the internalized tx already exist for the user, so internalize merges.
func (f *internalizeFixture) givenStoredTx(status wdk.TxStatus) {
	f.txRepo.stored = &pkgentity.Transaction{ID: 42, UserID: internalizeTestUserID, Status: status}
}

// paymentArgs returns wallet-payment args for an unmined tx whose only input is mined, so the BEEF
// anchors itself and needs no storage reads to verify.
func paymentArgs(t *testing.T) (wdk.InternalizeActionArgs, string) {
	t.Helper()
	args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)
	return args, atomicSubjectTxID(t, args.Tx)
}

// zeroSatoshiPaymentArgs is paymentArgs with a 0-satoshi payment output.
func zeroSatoshiPaymentArgs(t *testing.T) (wdk.InternalizeActionArgs, string) {
	t.Helper()
	atomicBeef, err := txtestabilities.GivenTX().WithInput(1000).WithP2PKHOutput(0).TX().AtomicBEEF(false)
	require.NoError(t, err)

	args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)
	args.Tx = atomicBeef
	return args, atomicSubjectTxID(t, atomicBeef)
}

func atomicSubjectTxID(t *testing.T, atomicBeef []byte) string {
	t.Helper()
	_, txID, err := transaction.NewBeefFromAtomicBytes(atomicBeef)
	require.NoError(t, err)
	return txID.String()
}
