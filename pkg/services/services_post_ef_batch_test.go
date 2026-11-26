package services_test

import (
	"context"
	"encoding/hex"
	"errors"
	"sync"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// recordingBroadcaster is a custom broadcaster that records every PostEF, PostEFBatch and
// PostTX call and accepts every transaction.
type recordingBroadcaster struct {
	mu         sync.Mutex
	efHexCalls []string // txIDs, one per PostEF call
	efHexes    map[string]string
	batchCalls [][]services.EFTransaction
	txCalls    int
	batchErr   error
}

func (r *recordingBroadcaster) PostEF(_ context.Context, efHex, txID string) (*wdk.PostedTxID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.efHexCalls = append(r.efHexCalls, txID)
	if r.efHexes == nil {
		r.efHexes = map[string]string{}
	}
	r.efHexes[txID] = efHex
	return &wdk.PostedTxID{TxID: txID, Result: wdk.PostedTxIDResultSuccess, Data: "ef:" + efHex}, nil
}

func (r *recordingBroadcaster) PostEFBatch(_ context.Context, txs []services.EFTransaction) ([]*wdk.PostedTxID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.batchCalls = append(r.batchCalls, txs)
	if r.batchErr != nil {
		return nil, r.batchErr
	}
	results := make([]*wdk.PostedTxID, len(txs))
	for i, tx := range txs {
		results[i] = &wdk.PostedTxID{TxID: tx.TxID, Result: wdk.PostedTxIDResultSuccess, Data: "ef:" + hex.EncodeToString(tx.EF)}
	}
	return results, nil
}

func (r *recordingBroadcaster) PostTX(_ context.Context, rawTx []byte) (*wdk.PostedTxID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.txCalls++
	return &wdk.PostedTxID{Result: wdk.PostedTxIDResultSuccess, Data: "raw:" + hex.EncodeToString(rawTx)}, nil
}

// postEFOnly, postEFBatchOnly, both and postTXOnly expose a subset of the broadcaster's methods.
func (r *recordingBroadcaster) postEFOnly() services.Implementation {
	return services.Implementation{PostEF: r.PostEF}
}

func (r *recordingBroadcaster) postEFBatchOnly() services.Implementation {
	return services.Implementation{PostEFBatch: r.PostEFBatch}
}

func (r *recordingBroadcaster) both() services.Implementation {
	return services.Implementation{PostEF: r.PostEF, PostEFBatch: r.PostEFBatch}
}

func (r *recordingBroadcaster) postTXOnly() services.Implementation {
	return services.Implementation{PostTX: r.PostTX}
}

// givenParentAndChild returns a BEEF of a child and its unmined parent, and both txs.
func givenParentAndChild(t *testing.T) (beef *sdk.Beef, parent, child *sdk.Transaction) {
	t.Helper()
	parent = txtestabilities.GivenTX().
		WithSender(txtestabilities.Alice).WithRecipient(txtestabilities.Alice).
		WithInput(100).
		WithP2PKHOutput(99).
		TX()
	child = txtestabilities.GivenTX().WithInputFromUTXO(parent, 0).WithP2PKHOutput(98).TX()
	beef, err := sdk.NewBeefFromTransaction(child)
	require.NoError(t, err)
	return beef, parent, child
}

// onlyCustomBroadcasters builds services that broadcast through the given custom
// implementations alone: Arcade, ARC, WhatsOnChain and Bitails are disabled.
func onlyCustomBroadcasters(t *testing.T, opts ...func(*services.Options)) *services.WalletServices {
	t.Helper()
	given := testservices.GivenServices(t)
	return given.Services().
		Config(
			testservices.WithEnabledArcade(false),
			testservices.WithEnabledARC(false),
			testservices.WithEnabledWoC(false),
			testservices.WithEnabledBitails(false),
		).
		Opts(opts...).
		New()
}

func resultSummary(results wdk.PostFromBeefResult) []string {
	summary := make([]string, 0, len(results))
	for _, r := range results {
		line := r.Name
		if r.Error != nil {
			line += " error=" + r.Error.Error()
		}
		if r.PostedBEEFResult != nil {
			for _, posted := range r.PostedBEEFResult.TxIDResults {
				line += " tx=" + posted.TxID + " result=" + string(posted.Result) + " " + posted.Data
			}
		}
		summary = append(summary, line)
	}
	return summary
}

func TestPostFromBEEF_PostEFBatch(t *testing.T) {
	t.Run("a batch broadcaster gets every transaction in one call, as EF bytes, parents first", func(t *testing.T) {
		// given:
		beef, parent, child := givenParentAndChild(t)
		perTx, batch, raw := &recordingBroadcaster{}, &recordingBroadcaster{}, &recordingBroadcaster{}

		svc := onlyCustomBroadcasters(t,
			services.WithCustomImplementation("per-tx", perTx.postEFOnly()),
			services.WithCustomImplementation("batch", batch.postEFBatchOnly()),
			services.WithCustomImplementation("raw", raw.postTXOnly()),
		)
		txIDs := []string{parent.TxID().String(), child.TxID().String()}

		// when:
		results, err := svc.PostFromBEEF(t.Context(), beef, txIDs)

		// then: one batch call with both transactions, in order, as the EF the per-tx broadcaster got as hex
		require.NoError(t, err)
		require.Len(t, batch.batchCalls, 1)
		require.Len(t, batch.batchCalls[0], 2)
		for i, txID := range txIDs {
			assert.Equal(t, txID, batch.batchCalls[0][i].TxID)
			assert.Equal(t, perTx.efHexes[txID], hex.EncodeToString(batch.batchCalls[0][i].EF))
		}
		assert.Equal(t, txIDs, perTx.efHexCalls)
		assert.Equal(t, 2, raw.txCalls)

		// and: per transaction, the PostEF results in registration order, then the PostTX ones
		require.Len(t, results, 6)
		for i, txID := range txIDs {
			assert.Equal(t, "per-tx", results[3*i].Name)
			assert.Equal(t, "batch", results[3*i+1].Name)
			assert.Equal(t, "raw", results[3*i+2].Name)
			assert.Equal(t, txID, results[3*i].PostedBEEFResult.TxIDResults[0].TxID)
			assert.Equal(t, txID, results[3*i+1].PostedBEEFResult.TxIDResults[0].TxID)
		}
	})

	t.Run("a batch broadcaster gives the results its PostEF would have given", func(t *testing.T) {
		// given:
		beef, parent, child := givenParentAndChild(t)
		txIDs := []string{parent.TxID().String(), child.TxID().String()}

		viaPostEF := onlyCustomBroadcasters(t, services.WithCustomImplementation("custom", (&recordingBroadcaster{}).postEFOnly()))
		viaBatch := onlyCustomBroadcasters(t, services.WithCustomImplementation("custom", (&recordingBroadcaster{}).postEFBatchOnly()))

		// when:
		want, err := viaPostEF.PostFromBEEF(t.Context(), beef, txIDs)
		require.NoError(t, err)
		got, err := viaBatch.PostFromBEEF(t.Context(), beef, txIDs)
		require.NoError(t, err)

		// then:
		assert.Equal(t, resultSummary(want), resultSummary(got))
		assert.Equal(t, want.Aggregated(txIDs), got.Aggregated(txIDs))
	})

	t.Run("an implementation with PostEF and PostEFBatch is called only through PostEFBatch", func(t *testing.T) {
		// given:
		beef, parent, child := givenParentAndChild(t)
		custom := &recordingBroadcaster{}
		svc := onlyCustomBroadcasters(t, services.WithCustomImplementation("custom", custom.both()))

		// when:
		results, err := svc.PostFromBEEF(t.Context(), beef, []string{parent.TxID().String(), child.TxID().String()})

		// then:
		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Len(t, custom.batchCalls, 1)
		assert.Empty(t, custom.efHexCalls)
	})

	t.Run("a failed batch call is an error result of that broadcaster for every transaction", func(t *testing.T) {
		// given:
		beef, parent, child := givenParentAndChild(t)
		failing := &recordingBroadcaster{batchErr: errors.New("batch endpoint down")}
		perTx := &recordingBroadcaster{}
		svc := onlyCustomBroadcasters(t,
			services.WithCustomImplementation("failing", failing.postEFBatchOnly()),
			services.WithCustomImplementation("per-tx", perTx.postEFOnly()),
		)
		txIDs := []string{parent.TxID().String(), child.TxID().String()}

		// when:
		results, err := svc.PostFromBEEF(t.Context(), beef, txIDs)

		// then:
		require.NoError(t, err)
		require.Len(t, results, 4)
		for i := range txIDs {
			assert.Equal(t, "failing", results[2*i].Name)
			require.ErrorContains(t, results[2*i].Error, "batch endpoint down")
			assert.Equal(t, "per-tx", results[2*i+1].Name)
			assert.NoError(t, results[2*i+1].Error)
		}
	})

	t.Run("with a PostEF modifier, a batch-only broadcaster is a PostEF of one transaction", func(t *testing.T) {
		// given:
		beef, parent, child := givenParentAndChild(t)
		batch := &recordingBroadcaster{}
		var seen []string
		svc := onlyCustomBroadcasters(t,
			services.WithCustomImplementation("batch", batch.postEFBatchOnly()),
			services.WithPostEFMethodsModifier(func(original []services.Named[services.PostEFFunc]) []services.Named[services.PostEFFunc] {
				for _, it := range original {
					seen = append(seen, it.Name)
				}
				return original
			}),
		)
		txIDs := []string{parent.TxID().String(), child.TxID().String()}

		// when:
		results, err := svc.PostFromBEEF(t.Context(), beef, txIDs)

		// then: the modifier saw it, and it was called once per transaction
		require.NoError(t, err)
		assert.Equal(t, []string{"batch"}, seen)
		require.Len(t, batch.batchCalls, 2)
		for i, txID := range txIDs {
			require.Len(t, batch.batchCalls[i], 1)
			assert.Equal(t, txID, batch.batchCalls[i][0].TxID)
		}
		assert.Len(t, results, 2)
	})

	t.Run("with a PostEF modifier, an implementation with both is called through PostEF", func(t *testing.T) {
		// given:
		beef, parent, _ := givenParentAndChild(t)
		custom := &recordingBroadcaster{}
		svc := onlyCustomBroadcasters(t,
			services.WithCustomImplementation("custom", custom.both()),
			services.WithPostEFMethodsModifier(func(original []services.Named[services.PostEFFunc]) []services.Named[services.PostEFFunc] {
				return original
			}),
		)

		// when:
		_, err := svc.PostFromBEEF(t.Context(), beef, []string{parent.TxID().String()})

		// then:
		require.NoError(t, err)
		assert.Equal(t, []string{parent.TxID().String()}, custom.efHexCalls)
		assert.Empty(t, custom.batchCalls)
	})
}
