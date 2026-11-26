package repo_test

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"slices"
	"testing"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/dbretry"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// The BEEF build parses each stored raw tx once and merges that parse, where it used to parse
// a raw tx to find its parents, again to inspect it and a third time inside MergeRawTx. The
// result must not change: this test runs the build and the previous build
// (ReferenceGetBEEFForTxIDs) over the same storage and compares what they produce.

// singleParseTx builds a tx spending output 0 of each parent, or output vout[i] when given,
// with outputs fan-out outputs.
func singleParseTx(lockTime uint32, outputs int, parents []*transaction.Transaction, vouts ...uint32) *transaction.Transaction {
	tx := &transaction.Transaction{Version: 1, LockTime: lockTime}
	for i, p := range parents {
		vout := uint32(0)
		if i < len(vouts) {
			vout = vouts[i]
		}
		tx.Inputs = append(tx.Inputs, &transaction.TransactionInput{
			SourceTXID:        p.TxID(),
			SourceTransaction: p,
			SourceTxOutIndex:  vout,
			SequenceNumber:    0xffffffff,
		})
	}
	for i := range outputs {
		tx.Outputs = append(tx.Outputs, &transaction.TransactionOutput{
			Satoshis:      uint64(1000 + i),
			LockingScript: &script.Script{script.OpTRUE},
		})
	}
	return tx
}

func storeSingleParseKnownTx(t *testing.T, db *database.Database, tx *transaction.Transaction, status wdk.ProvenTxReqStatus, merklePath []byte, inputBEEF []byte) {
	t.Helper()
	require.NoError(t, db.DB.Create(&models.KnownTx{
		TxID:       tx.TxID().String(),
		Status:     status,
		RawTx:      tx.Bytes(),
		InputBeef:  inputBEEF,
		MerklePath: merklePath,
	}).Error)
}

func beefOf(t *testing.T, txs ...*transaction.Transaction) []byte {
	t.Helper()
	beef := transaction.NewBeefV2()
	for _, tx := range txs {
		_, err := beef.MergeTransaction(tx)
		require.NoError(t, err)
	}
	b, err := beef.Bytes()
	require.NoError(t, err)
	return b
}

// canonicalBEEF renders a BEEF independently of map order: per txid its data format, raw tx and
// proof, and the sorted list of BUMPs.
func canonicalBEEF(beef *transaction.Beef) []string {
	out := make([]string, 0, len(beef.Transactions)+len(beef.BUMPs))
	for txid, beefTx := range beef.Transactions {
		entry := fmt.Sprintf("%s format=%d", txid.String(), beefTx.DataFormat)
		if beefTx.KnownTxID != nil {
			entry += " known=" + beefTx.KnownTxID.String()
		}
		if beefTx.Transaction != nil {
			entry += " raw=" + hex.EncodeToString(beefTx.Transaction.Bytes())
			if beefTx.Transaction.MerklePath != nil {
				entry += " proof=" + beefTx.Transaction.MerklePath.Hex()
			}
		}
		if beefTx.DataFormat == transaction.RawTxAndBumpIndex {
			entry += " bump=" + beef.BUMPs[beefTx.BumpIndex].Hex()
		}
		out = append(out, entry)
	}
	for _, bump := range beef.BUMPs {
		out = append(out, "BUMP "+bump.Hex())
	}
	slices.Sort(out)
	return out
}

func TestGetBEEFForTxIDs_SingleParseMatchesPreviousBuild(t *testing.T) {
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	// given: a mined grandparent, two unmined parents (one a wide fan-out), an external tx known
	// only from input beef, and subjects spending them; plus a subject of an invalid tx and one
	// whose parent storage has never seen (the build fails for it).
	grandparent := singleParseTx(1, 3, nil)
	grandparentPath := testutils.MockValidMerklePath(t, grandparent.TxID().String(), 1000)
	require.NoError(t, grandparent.AddMerkleProof(&grandparentPath))

	fanOut := singleParseTx(2, 200, []*transaction.Transaction{grandparent}, 0)
	sibling := singleParseTx(3, 2, []*transaction.Transaction{grandparent}, 1)
	external := singleParseTx(4, 1, nil)
	externalPath := testutils.MockValidMerklePath(t, external.TxID().String(), 1001)
	require.NoError(t, external.AddMerkleProof(&externalPath))

	wide := singleParseTx(5, 2, []*transaction.Transaction{fanOut, sibling, external}, 7, 0, 0)
	narrow := singleParseTx(6, 2, []*transaction.Transaction{fanOut}, 199)
	grandchild := singleParseTx(7, 1, []*transaction.Transaction{narrow}, 0)
	invalidParent := singleParseTx(8, 1, []*transaction.Transaction{grandparent}, 2)
	ofInvalid := singleParseTx(9, 1, []*transaction.Transaction{invalidParent}, 0)
	unknown := singleParseTx(10, 1, nil)
	orphan := singleParseTx(11, 1, []*transaction.Transaction{unknown}, 0)

	storeSingleParseKnownTx(t, db, grandparent, wdk.ProvenTxStatusCompleted, grandparentPath.Bytes(), nil)
	storeSingleParseKnownTx(t, db, fanOut, wdk.ProvenTxStatusUnmined, nil, beefOf(t, grandparent))
	storeSingleParseKnownTx(t, db, sibling, wdk.ProvenTxStatusUnmined, nil, beefOf(t, grandparent))
	storeSingleParseKnownTx(t, db, wide, wdk.ProvenTxStatusUnprocessed, nil, beefOf(t, fanOut, sibling, external))
	storeSingleParseKnownTx(t, db, narrow, wdk.ProvenTxStatusUnmined, nil, beefOf(t, fanOut))
	storeSingleParseKnownTx(t, db, grandchild, wdk.ProvenTxStatusUnprocessed, nil, beefOf(t, narrow))
	storeSingleParseKnownTx(t, db, invalidParent, wdk.ProvenTxStatusInvalid, nil, beefOf(t, grandparent))
	storeSingleParseKnownTx(t, db, ofInvalid, wdk.ProvenTxStatusUnprocessed, nil, beefOf(t, invalidParent))
	storeSingleParseKnownTx(t, db, orphan, wdk.ProvenTxStatusUnprocessed, nil, nil)

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())

	id := func(tx *transaction.Transaction) string { return tx.TxID().String() }

	subjectSets := map[string][]string{
		"wide":               {id(wide)},
		"narrow":             {id(narrow)},
		"grandchild":         {id(grandchild)},
		"wide and narrow":    {id(wide), id(narrow)},
		"narrow and wide":    {id(narrow), id(wide)},
		"grandchild, narrow": {id(grandchild), id(narrow)},
		"parent and child":   {id(fanOut), id(narrow)},
		"child and parent":   {id(narrow), id(fanOut)},
		"of invalid parent":  {id(ofInvalid)},
		"orphan":             {id(orphan)},
		"wide and orphan":    {id(wide), id(orphan)},
	}

	options := map[string]func() []entity.GetBEEFOption{
		"defaults":       func() []entity.GetBEEFOption { return nil },
		"direct sources": func() []entity.GetBEEFOption { return []entity.GetBEEFOption{entity.WithDirectSourcesOnly()} },
		"anchored direct sources": func() []entity.GetBEEFOption {
			return []entity.GetBEEFOption{entity.WithDirectSourcesOnly(), entity.WithAnchoredParents()}
		},
		"min proof level 1": func() []entity.GetBEEFOption { return []entity.GetBEEFOption{entity.WithMinProofLevel(1)} },
		"min proof level 3": func() []entity.GetBEEFOption { return []entity.GetBEEFOption{entity.WithMinProofLevel(3)} },
		"known txids": func() []entity.GetBEEFOption {
			return []entity.GetBEEFOption{entity.WithKnownTxIDs(id(sibling), id(narrow))}
		},
		"trust self": func() []entity.GetBEEFOption { return []entity.GetBEEFOption{entity.WithTrustSelf(sdk.TrustSelfKnown)} },
		"invalid filtered out": func() []entity.GetBEEFOption {
			return []entity.GetBEEFOption{entity.WithStatusesToFilterOut(wdk.ProvenTxStatusInvalid)}
		},
		"invalid filtered out, direct sources": func() []entity.GetBEEFOption {
			return []entity.GetBEEFOption{entity.WithStatusesToFilterOut(wdk.ProvenTxStatusInvalid), entity.WithDirectSourcesOnly()}
		},
		"merge into a beef holding the fan-out": func() []entity.GetBEEFOption {
			into := transaction.NewBeefV2()
			_, err := into.MergeRawTx(fanOut.Bytes(), nil)
			require.NoError(t, err)
			return []entity.GetBEEFOption{entity.WithMergeToBEEF(into)}
		},
		"merge into a beef holding the fan-out's txid only": func() []entity.GetBEEFOption {
			into := transaction.NewBeefV2()
			into.MergeTxidOnly(fanOut.TxID())
			return []entity.GetBEEFOption{entity.WithMergeToBEEF(into)}
		},
		"merge into a beef holding the fan-out's txid only, direct sources": func() []entity.GetBEEFOption {
			into := transaction.NewBeefV2()
			into.MergeTxidOnly(fanOut.TxID())
			return []entity.GetBEEFOption{entity.WithMergeToBEEF(into), entity.WithDirectSourcesOnly()}
		},
		"merge into a beef holding the fan-out, direct sources": func() []entity.GetBEEFOption {
			into := transaction.NewBeefV2()
			_, err := into.MergeRawTx(fanOut.Bytes(), nil)
			require.NoError(t, err)
			return []entity.GetBEEFOption{entity.WithMergeToBEEF(into), entity.WithDirectSourcesOnly()}
		},
	}

	for subjectsName, subjects := range subjectSets {
		for optionsName, opts := range options {
			t.Run(subjectsName+"/"+optionsName, func(t *testing.T) {
				// when:
				got, gotErr := repos.GetBEEFForTxIDs(t.Context(), seq.FromSlice(subjects), opts()...)
				want, wantErr := repos.ReferenceGetBEEFForTxIDs(t.Context(), seq.FromSlice(subjects), opts()...)

				// then:
				if wantErr != nil {
					require.EqualError(t, gotErr, wantErr.Error())
					return
				}
				require.NoError(t, gotErr)
				require.Equal(t, canonicalBEEF(want), canonicalBEEF(got))

				wantBytes, err := want.Bytes()
				require.NoError(t, err)
				gotBytes, err := got.Bytes()
				require.NoError(t, err)
				require.Len(t, gotBytes, len(wantBytes))
			})
		}
	}
}

// BenchmarkGetBEEFForTxIDs_WideParent builds the broadcast BEEF of a tx spending one output of
// an unmined 1000-output fan-out, the shape of a throughput-strategy (fuel) spend.
//
//	go test -run '^$' -bench GetBEEFForTxIDs_WideParent -benchmem ./pkg/internal/storage/repo/
func BenchmarkGetBEEFForTxIDs_WideParent(b *testing.B) {
	db, cleanup := dbfixtures.TestDatabase(b)
	defer cleanup()
	db.DB.Logger = database.NewSlogGormLogger(slog.New(slog.DiscardHandler))

	funding := singleParseTx(1, 1, nil)
	fanOut := singleParseTx(2, 1000, []*transaction.Transaction{funding})
	spend := singleParseTx(3, 2, []*transaction.Transaction{fanOut}, 500)

	for _, tx := range []*transaction.Transaction{funding, fanOut, spend} {
		require.NoError(b, db.DB.Create(&models.KnownTx{
			TxID:   tx.TxID().String(),
			Status: wdk.ProvenTxStatusUnmined,
			RawTx:  tx.Bytes(),
		}).Error)
	}

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	subjects := []string{spend.TxID().String()}

	for name, opts := range map[string][]entity.GetBEEFOption{
		"direct sources": {entity.WithDirectSourcesOnly()},
		"full ancestry":  nil,
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := repos.GetBEEFForTxIDs(b.Context(), seq.FromSlice(subjects), opts...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
