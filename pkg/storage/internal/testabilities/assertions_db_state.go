package testabilities

import (
	"context"
	"fmt"
	"maps"
	"testing"
	"time"

	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/crud"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

type StorageReader interface {
	KnownTxEntity() crud.KnownTx
	TransactionEntity() crud.Transaction
	FindOrInsertUser(ctx context.Context, identityKey string) (*wdk.FindOrInsertUserResponse, error)
	ListOutputs(ctx context.Context, auth wdk.AuthID, args wdk.ListOutputsArgs) (*wdk.ListOutputsResult, error)
	ListActions(ctx context.Context, auth wdk.AuthID, args wdk.ListActionsArgs) (*wdk.ListActionsResult, error)
	CreateAction(ctx context.Context, auth wdk.AuthID, args wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, error)
}

type DBStateAssertion interface {
	HasKnownTXs(txIDs ...string) DBStateAssertion
	HasKnownTX(txID string) KnownTxAssertion
	HasUserTransactionByReference(user testusers.User, reference string) UserTransactionAssertion
	HasUserTransactionByTxID(user testusers.User, txID string) UserTransactionAssertion

	HasUserTransactionsByTxIDsWithStatus(user testusers.User, status wdk.TxStatus, txIDs ...string)

	AllOutputs(user testusers.User) OutputsListAssertion
	Outputs(user testusers.User, basketName string) OutputsListAssertion
	WaitForTxStatusByReference(
		user testusers.User,
		reference string,
		status wdk.TxStatus,
		timeout time.Duration,
	)

	// CanCreateActionForSatoshis - is the only way to check if UserUTXOs have been created for the user,
	// by attempting to create an action for the user (which requires UserUTXOs to exist).
	// NOTE: No other methods should be called before this one, as it changes DB state.
	CanCreateActionForSatoshis(user testusers.User, satoshi uint64) //
}

type KnownTxAssertion interface {
	WithStatus(state wdk.ProvenTxReqStatus) KnownTxAssertion
	WithAttempts(attempts uint64) KnownTxAssertion
	IsMined() KnownTxAssertion
	NotMined() KnownTxAssertion
	HasRawTx() KnownTxAssertion
	IsNotified(expected bool) KnownTxAssertion
	WithBlockHeight(expected *uint32) KnownTxAssertion
	WithMerkleRoot(expected *string) KnownTxAssertion
	WithBlockHash(expected *string) KnownTxAssertion
	TxNotes(assertion func(TxNotesAssertion)) KnownTxAssertion
}

type UserTransactionAssertion interface {
	WithStatus(state wdk.TxStatus) UserTransactionAssertion
	WithTxID(txID string) UserTransactionAssertion
	WithoutTxID() UserTransactionAssertion
	WithLabels(labels ...string) UserTransactionAssertion
}

type OutputsListAssertion interface {
	WithCount(expected int) OutputsListAssertion
	WithCountHavingTxID(expected int) OutputsListAssertion
	WithCountHavingTags(expected int, tags ...string) OutputsListAssertion
}

type TxNotesAssertion interface {
	Count(expected int) TxNotesAssertion
	Note(what string, userID *int, attrs map[string]any) TxNotesAssertion
}

func ThenDBState(t testing.TB, storage StorageReader) DBStateAssertion {
	t.Helper()

	if storage == nil {
		require.FailNow(t, "Storage cannot be nil")
	}

	return &dbStateAssertion{
		TB:      t,
		storage: storage,
	}
}

type dbStateAssertion struct {
	testing.TB

	storage StorageReader
}

func (d *dbStateAssertion) HasUserTransactionsByTxIDsWithStatus(user testusers.User, status wdk.TxStatus, txIDs ...string) {
	for _, txID := range txIDs {
		d.HasUserTransactionByTxID(user, txID).WithStatus(status)
	}
}

func (d *dbStateAssertion) userIDByIdentityKey(identityKey string) int {
	d.Helper()

	addUserResult, err := d.storage.FindOrInsertUser(d.Context(), identityKey)
	require.NoError(d, err, "Failed to find user by identity key: %s", identityKey)
	require.False(d, addUserResult.IsNew, "Expected the user to already exist, but it was created: %s", identityKey)

	return addUserResult.User.UserID
}

func (d *dbStateAssertion) HasKnownTXs(txIDs ...string) DBStateAssertion {
	d.Helper()

	missingTXs := map[string]struct{}{}

	for _, txID := range txIDs {
		found, err := d.storage.KnownTxEntity().Read().TxID(txID).Find(d.Context())
		require.NoError(d, err)

		if len(found) == 0 {
			missingTXs[txID] = struct{}{}
		}
	}

	if len(missingTXs) != 0 {
		missingIDs := seq.Collect(maps.Keys(missingTXs))
		assert.Failf(d, "Expected to find all the transactions", "missing transaction IDs: %v", missingIDs)
	}

	return d
}

func (d *dbStateAssertion) HasKnownTX(txID string) KnownTxAssertion {
	d.Helper()

	found, err := d.storage.KnownTxEntity().Read().
		TxID(txID).
		IncludeHistoryNotes().
		Find(d.Context())
	require.NoError(d, err)

	if len(found) == 0 {
		require.Failf(d, "Expected to find the transaction", "transaction ID: %s", txID)
		return nil
	}

	knownTx := found[0]
	assert.Equal(d, txID, knownTx.TxID, "Expected known transaction to have the same TxID as the one requested")

	return &knownTxAssertion{
		TB:      d.TB,
		knownTx: knownTx,
	}
}

type knownTxAssertion struct {
	testing.TB

	knownTx *pkgentity.KnownTx
}

func (d *knownTxAssertion) WithStatus(state wdk.ProvenTxReqStatus) KnownTxAssertion {
	d.Helper()
	assert.Equal(d, state, d.knownTx.Status, "Expected known transaction to have the status %s", state)
	return d
}

func (d *knownTxAssertion) IsMined() KnownTxAssertion {
	d.Helper()
	assert.NotNil(d, d.knownTx.BlockHeight)
	assert.NotEmpty(d, d.knownTx.MerklePath)
	assert.NotEmpty(d, d.knownTx.MerkleRoot)
	assert.NotEmpty(d, d.knownTx.BlockHash)
	return d
}

func (d *knownTxAssertion) NotMined() KnownTxAssertion {
	d.Helper()
	assert.Nil(d, d.knownTx.BlockHeight)
	assert.Empty(d, d.knownTx.MerklePath)
	assert.Empty(d, d.knownTx.MerkleRoot)
	assert.Empty(d, d.knownTx.BlockHash)
	assert.NotEqual(d, wdk.ProvenTxStatusCompleted, d.knownTx.Status)
	return d
}

func (d *knownTxAssertion) HasRawTx() KnownTxAssertion {
	d.Helper()
	assert.NotEmpty(d, d.knownTx.RawTx, "Expected known transaction to have a non-empty RawTx")
	return d
}

func (d *knownTxAssertion) TxNotes(assertion func(TxNotesAssertion)) KnownTxAssertion {
	for _, note := range d.knownTx.TxNotes {
		assert.Equal(d, d.knownTx.TxID, note.TxID, "Expected TxNote to have the same TxID as the known transaction")
	}

	assertion(&txNotesAssertion{
		TB:      d.TB,
		txNotes: d.knownTx.TxNotes,
	})

	return d
}

func (d *knownTxAssertion) WithAttempts(expected uint64) KnownTxAssertion {
	d.Helper()
	assert.Equal(d, expected, d.knownTx.Attempts, "Expected known transaction to have %d Attempts", expected)
	return d
}

func (d *knownTxAssertion) WithBlockHeight(expected *uint32) KnownTxAssertion {
	d.Helper()
	assert.Equal(d, expected, d.knownTx.BlockHeight, "Expected known tx to have BlockHeight = %v", expected)
	return d
}

func (d *knownTxAssertion) WithMerkleRoot(expected *string) KnownTxAssertion {
	d.Helper()
	assert.Equal(d, expected, d.knownTx.MerkleRoot, "Expected MerkleRoot = %v", expected)
	return d
}

func (d *knownTxAssertion) WithBlockHash(expected *string) KnownTxAssertion {
	d.Helper()
	assert.Equal(d, expected, d.knownTx.BlockHash, "Expected BlockHash = %v", expected)
	return d
}

func (d *knownTxAssertion) IsNotified(expected bool) KnownTxAssertion {
	d.Helper()
	assert.Equal(d, expected, d.knownTx.Notified, "Expected known transaction to have Notified = %v", expected)
	return d
}

type txNotesAssertion struct {
	testing.TB

	txNotes      []*pkgentity.TxHistoryNote
	currentIndex int
}

func (d *txNotesAssertion) Count(expected int) TxNotesAssertion {
	d.Helper()
	if !assert.NotNil(d, d.txNotes, "Expected known transaction to have TxNotes") {
		return d
	}

	assert.Len(d, d.txNotes, expected, "Expected known transaction to have %d TxNotes, but got %d", expected, len(d.txNotes))
	return d
}

func (d *txNotesAssertion) Note(what string, userID *int, attrs map[string]any) TxNotesAssertion {
	d.Helper()

	if !assert.NotNil(d, d.txNotes, "Expected known transaction to have TxNotes") {
		return d
	}

	if d.currentIndex >= len(d.txNotes) {
		assert.Failf(d, "No more TxNotes available", "Expected to find a TxNote with what=%s, userID=%v, attrs=%v", what, userID, attrs)
		return d
	}

	note := d.txNotes[d.currentIndex]
	d.currentIndex++

	assert.Equal(d, what, note.What, "Expected TxNote to have the same 'What' as requested")
	assert.Equal(d, userID, note.UserID, "Expected TxNote to have the same 'UserID' as requested")
	var zeroTime time.Time
	assert.NotEqual(d, zeroTime, note.When, "Expected TxNote to have a non-zero 'When' timestamp")

	for k, v := range attrs {
		val, ok := note.Attributes[k]
		assert.True(d, ok, "Expected TxNote to have attribute '%s'", k)
		assert.Equal(d, fmt.Sprintf("%v", v), fmt.Sprintf("%v", val), "Expected TxNote to have the same value for attribute '%s'", k)
	}

	return d
}

func (d *dbStateAssertion) getUserTransactionByReference(user testusers.User, reference string) *pkgentity.Transaction {
	d.Helper()

	userID := d.userIDByIdentityKey(user.IdentityKey(d))
	txs, err := d.storage.TransactionEntity().Read().
		UserID().Equals(userID).
		Reference().Equals(reference).
		Find(d.Context())
	require.NoError(d, err)
	require.Len(d, txs, 1)

	tx := txs[0]
	assert.Equal(d, reference, tx.Reference, "Expected user transaction to have the same Reference as the one requested")

	return tx
}

func (d *dbStateAssertion) HasUserTransactionByReference(user testusers.User, reference string) UserTransactionAssertion {
	d.Helper()

	tx := d.getUserTransactionByReference(user, reference)

	return &userTransactionAssertion{
		TB:          d.TB,
		transaction: tx,
	}
}

func (d *dbStateAssertion) WaitForTxStatusByReference(
	user testusers.User,
	reference string,
	status wdk.TxStatus,
	timeout time.Duration,
) {
	d.Helper()

	condition := func() bool {
		currentStatus := d.getUserTransactionByReference(user, reference).Status
		return currentStatus == status
	}

	if condition() {
		return
	}

	assert.Eventually(d, condition, timeout, 500*time.Millisecond, "Expected user transaction with reference '%s' to have status '%s' within %s", reference, status, timeout)
}

func (d *dbStateAssertion) HasUserTransactionByTxID(user testusers.User, txID string) UserTransactionAssertion {
	d.Helper()

	userID := d.userIDByIdentityKey(user.IdentityKey(d))
	txs, err := d.storage.TransactionEntity().Read().
		UserID().Equals(userID).
		TxID().Equals(txID).
		Find(d.Context())
	require.NoError(d, err)
	require.Len(d, txs, 1)

	tx := txs[0]
	assert.Equal(d, txID, *tx.TxID, "Expected user transaction to have the same TxID as the one requested")

	return &userTransactionAssertion{
		TB:          d.TB,
		transaction: tx,
	}
}

type userTransactionAssertion struct {
	testing.TB

	transaction *pkgentity.Transaction
}

func (d *userTransactionAssertion) WithStatus(status wdk.TxStatus) UserTransactionAssertion {
	d.Helper()
	assert.Equal(d, status, d.transaction.Status, "Expected user transaction to have the status %s", status)
	return d
}

func (d *userTransactionAssertion) WithTxID(txID string) UserTransactionAssertion {
	d.Helper()
	if !assert.NotNil(d, d.transaction.TxID, "Expected user transaction to have a non-empty TxID") {
		return d
	}

	assert.Equal(d, txID, *d.transaction.TxID, "Expected user transaction to have the same TxID as the one requested")
	return d
}

func (d *userTransactionAssertion) WithoutTxID() UserTransactionAssertion {
	d.Helper()
	assert.Nil(d, d.transaction.TxID)
	return d
}

func (d *userTransactionAssertion) WithLabels(labels ...string) UserTransactionAssertion {
	d.Helper()
	assert.ElementsMatch(d, labels, d.transaction.Labels, "Expected user transaction to have the same Labels as the one requested")
	return d
}

func (d *dbStateAssertion) Outputs(user testusers.User, basketName string) OutputsListAssertion {
	d.Helper()

	userID := d.userIDByIdentityKey(user.IdentityKey(d))

	actions, err := d.storage.ListActions(d.Context(), wdk.AuthID{UserID: &userID}, wdk.ListActionsArgs{
		Limit:          primitives.PositiveIntegerDefault10Max10000(1000),
		IncludeOutputs: to.Ptr[primitives.BooleanDefaultFalse](true),
		LabelQueryMode: to.Ptr(defs.QueryModeAny),
	})
	require.NoError(d, err)

	var outputs []*outputInfo
	for _, action := range actions.Actions {
		for _, output := range action.Outputs {
			if basketName == "" || output.Basket == basketName {
				outputs = append(outputs, &outputInfo{
					WalletActionOutput: output,
					txID:               action.TxID,
				})
			}
		}
	}

	return &outputsListAssertion{
		TB:      d.TB,
		outputs: outputs,
	}
}

func (d *dbStateAssertion) AllOutputs(user testusers.User) OutputsListAssertion {
	return d.Outputs(user, "")
}

type outputsListAssertion struct {
	testing.TB

	outputs []*outputInfo
}

type outputInfo struct {
	wdk.WalletActionOutput

	txID string
}

func (d *outputsListAssertion) WithCount(expected int) OutputsListAssertion {
	d.Helper()
	assert.Len(d, d.outputs, expected, "Expected outputs list to have %d items, but got %d", expected, len(d.outputs))
	return d
}

func (d *outputsListAssertion) WithCountHavingTxID(expected int) OutputsListAssertion {
	d.Helper()
	count := seq.Count(seq.Filter(seq.FromSlice(d.outputs), func(output *outputInfo) bool {
		return output.txID != ""
	}))
	assert.Equal(d, expected, count, "Expected outputs list to have %d items with txID, but got %d", expected, count)
	return d
}

func (d *outputsListAssertion) WithCountHavingTags(expected int, tags ...string) OutputsListAssertion {
	d.Helper()

	lookup := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		lookup[tag] = struct{}{}
	}

	count := seq.Count(seq.Filter(seq.FromSlice(d.outputs), func(output *outputInfo) bool {
		contains := 0
		for _, tag := range output.Tags {
			if _, ok := lookup[tag]; ok {
				contains++
			}
			if contains >= len(tags) {
				return true
			}
		}
		return false
	}))
	assert.Equal(d, expected, count, "Expected outputs list to have %d items with tags %v, but got %d", expected, tags, count)
	return d
}

func (d *dbStateAssertion) CanCreateActionForSatoshis(user testusers.User, satoshis uint64) {
	d.Helper()

	userID := d.userIDByIdentityKey(user.IdentityKey(d))
	_, err := d.storage.CreateAction(d.Context(), wdk.AuthID{UserID: &userID}, wdk.ValidCreateActionArgs{
		Description: "test transaction",
		Outputs: []wdk.ValidCreateActionOutput{
			{
				LockingScript:      "76a9144b0d6cbef5a813d2d12dcec1de2584b250dc96a388ac",
				Satoshis:           primitives.SatoshiValue(satoshis),
				OutputDescription:  "outputBRC29",
				Basket:             nil,
				CustomInstructions: to.Ptr("{\"derivationPrefix\":\"Pr==\",\"derivationSuffix\":\"Su==\",\"type\":\"BRC29\"}"),
				Tags:               nil,
			},
		},
		LockTime: 0,
		Version:  1,
		Options: wdk.ValidCreateActionOptions{
			AcceptDelayedBroadcast: to.Ptr[primitives.BooleanDefaultTrue](false),
		},
		IsNewTx:                      true,
		IncludeAllSourceTransactions: true,
	})
	require.NoError(d.TB, err)
}
