package testabilities

import (
	"context"
	"maps"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type StorageReader interface {
	FindKnownTx(ctx context.Context, txID string) (*entity.KnownTx, error)
	FindUserTransactionByReference(ctx context.Context, userID int, reference string) (*entity.Transaction, error)
	FindOrInsertUser(ctx context.Context, identityKey string) (*wdk.FindOrInsertUserResponse, error)
	ListOutputs(ctx context.Context, auth wdk.AuthID, args wdk.ListOutputsArgs) (*wdk.ListOutputsResult, error)
	CreateAction(ctx context.Context, auth wdk.AuthID, args wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, error)
}

type DBStateAssertion interface {
	HasKnownTXs(txIDs ...string) DBStateAssertion
	HasKnownTX(txID string) KnownTxAssertion
	HasUserTransactionByReference(user testusers.User, txID string) UserTransactionAssertion
	AllOutputs(user testusers.User) OutputsListAssertion
	Outputs(user testusers.User, basketName string) OutputsListAssertion

	// CanCreateActionForSatoshis - is the only way to check if UserUTXOs have been created for the user,
	// by attempting to create an action for the user (which requires UserUTXOs to exist).
	// NOTE: No other methods should be called before this one, as it changes DB state.
	CanCreateActionForSatoshis(user testusers.User, satoshi uint64) //
}

type KnownTxAssertion interface {
	WithStatus(state wdk.ProvenTxReqStatus) KnownTxAssertion
	IsMined() KnownTxAssertion
	NotMined() KnownTxAssertion
	HasRawTx() KnownTxAssertion
}

type UserTransactionAssertion interface {
	WithStatus(state wdk.TxStatus) UserTransactionAssertion
	WithTxID(txID string) UserTransactionAssertion
	WithoutTxID() UserTransactionAssertion
	WithLabels(labels ...string) UserTransactionAssertion
}

type OutputsListAssertion interface {
	WithCount(expected int) OutputsListAssertion
	WithCountHavingOutpoint(expected int) OutputsListAssertion
	WithCountHavingTags(expected int, tags ...string) OutputsListAssertion
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
		knownTx, err := d.storage.FindKnownTx(d.Context(), txID)
		require.NoError(d.TB, err, txID)

		if knownTx == nil {
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

	knownTx, err := d.storage.FindKnownTx(d.Context(), txID)
	require.NoError(d.TB, err, txID)

	if knownTx == nil {
		require.Failf(d, "Expected to find the transaction", "transaction ID: %s", txID)
		return nil
	}

	assert.Equal(d, txID, knownTx.TxID, "Expected known transaction to have the same TxID as the one requested")

	return &knownTxAssertion{
		TB:      d.TB,
		knownTx: knownTx,
	}
}

type knownTxAssertion struct {
	testing.TB
	knownTx *entity.KnownTx
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
	assert.NotEqual(d, d.knownTx.Status, wdk.ProvenTxStatusCompleted)
	return d
}

func (d *knownTxAssertion) HasRawTx() KnownTxAssertion {
	d.Helper()
	assert.NotEmpty(d, d.knownTx.RawTx, "Expected known transaction to have a non-empty RawTx")
	return d
}

func (d *dbStateAssertion) HasUserTransactionByReference(user testusers.User, reference string) UserTransactionAssertion {
	d.Helper()

	userID := d.userIDByIdentityKey(user.IdentityKey(d))
	tx, err := d.storage.FindUserTransactionByReference(d.Context(), userID, reference) // UserID is not used here, so we pass 0
	require.NoError(d.TB, err)
	require.NotNil(d.TB, tx)

	assert.Equal(d, reference, tx.Reference, "Expected user transaction to have the same TxID as the one requested")

	return &userTransactionAssertion{
		TB:          d.TB,
		transaction: tx,
	}
}

type userTransactionAssertion struct {
	testing.TB
	transaction *entity.Transaction
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
	outputs, err := d.storage.ListOutputs(d.Context(), wdk.AuthID{UserID: &userID}, wdk.ListOutputsArgs{
		Limit:       1000,
		Basket:      primitives.StringUnder300(basketName),
		IncludeTags: true,
	})
	require.NoError(d.TB, err)

	return &outputsListAssertion{
		TB:      d.TB,
		outputs: outputs.Outputs,
	}
}

func (d *dbStateAssertion) AllOutputs(user testusers.User) OutputsListAssertion {
	return d.Outputs(user, "")
}

type outputsListAssertion struct {
	testing.TB
	outputs []*wdk.WalletOutput
}

func (d *outputsListAssertion) WithCount(expected int) OutputsListAssertion {
	d.Helper()
	assert.Len(d, d.outputs, expected, "Expected outputs list to have %d items, but got %d", expected, len(d.outputs))
	return d
}

func (d *outputsListAssertion) WithCountHavingOutpoint(expected int) OutputsListAssertion {
	d.Helper()
	count := seq.Count(seq.Filter(seq.FromSlice(d.outputs), func(output *wdk.WalletOutput) bool {
		err := output.Outpoint.Validate()
		return err == nil
	}))
	assert.Equal(d, expected, count, "Expected outputs list to have %d items with valid outpoints, but got %d", expected, count)
	return d
}

func (d *outputsListAssertion) WithCountHavingTags(expected int, tags ...string) OutputsListAssertion {
	d.Helper()

	lookup := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		lookup[tag] = struct{}{}
	}

	count := seq.Count(seq.Filter(seq.FromSlice(d.outputs), func(output *wdk.WalletOutput) bool {
		contains := 0
		for _, tag := range output.Tags {
			if _, ok := lookup[string(tag)]; ok {
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
