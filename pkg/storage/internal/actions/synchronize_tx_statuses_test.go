package actions

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestDoSynchronizeTxStatuses_NothingToSync(t *testing.T) {
	// given:
	f := newSyncFixture(t)

	// when:
	statuses, err := f.run(t.Context())

	// then: nothing is asked of the network, but the failure review still runs
	require.NoError(t, err)
	assert.Empty(t, statuses)
	assert.Equal(t, []string{"find", "review"}, f.rec.snapshot())
}

func TestDoSynchronizeTxStatuses_NothingToSyncStillFailsOnReviewError(t *testing.T) {
	// given:
	f := newSyncFixture(t)
	f.knownTx.reviewErr = errSyncBoom

	// when:
	statuses, err := f.run(t.Context())

	// then:
	require.ErrorIs(t, err, errSyncBoom)
	require.ErrorContains(t, err, "failed to review known tx statuses")
	assert.Nil(t, statuses)
}

func TestDoSynchronizeTxStatuses_NothingConfirmedDeepEnough(t *testing.T) {
	shallow := 0
	tests := map[string]struct {
		arrange func(f *syncFixture)
	}{
		"every candidate is below the required depth": {
			arrange: func(f *syncFixture) {
				f.services.details = []wdk.TxStatusDetail{{TxID: "a", Depth: &shallow, Status: wdk.ResultStatusForTxIDKnown.String()}}
			},
		},
		"the network gave no depth for the candidate": {
			arrange: func(f *syncFixture) {
				f.services.details = []wdk.TxStatusDetail{{TxID: "a", Status: wdk.ResultStatusForTxIDKnown.String()}}
			},
		},
		"the status service is unavailable": {
			arrange: func(f *syncFixture) { f.services.statusErr = errSyncBoom },
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			f := newSyncFixture(t)
			f.givenCandidates("a")
			test.arrange(f)

			// when:
			statuses, err := f.run(t.Context())

			// then: no proof is polled and no attempt is counted, but the review runs
			require.NoError(t, err)
			assert.Empty(t, statuses)
			assert.Equal(t, []string{"find", "depth", "review"}, f.rec.snapshot())
			assert.Empty(t, f.txRepo.referenceLookups)
		})
	}
}

func TestDoSynchronizeTxStatuses_ProvesMinedTx(t *testing.T) {
	// given:
	f := newSyncFixture(t)
	f.givenCandidates("a")
	f.givenMined("a", 850_001)

	// when:
	statuses, err := f.run(t.Context())

	// then: the tx is reported as completed with its proof, reference and labels
	require.NoError(t, err)
	mined := f.services.merkle["a"].result
	assert.Equal(t, []wdk.TxSynchronizedStatus{{
		TxID:        "a",
		Status:      wdk.ProvenTxStatusCompleted,
		Reference:   "ref-a",
		BlockHeight: 850_001,
		BlockHash:   "hash-a",
		MerkleRoot:  "root-a",
		MerklePath:  mined.MerklePath,
		Labels:      []string{"label-a"},
	}}, statuses)

	// and: the same proof is stored, with a note for each transaction row of the tx
	require.Len(t, f.knownTx.mined, 1)
	stored := f.knownTx.mined[0]
	assert.Equal(t, "a", stored.TxID)
	assert.Equal(t, uint32(850_001), stored.BlockHeight)
	assert.Equal(t, "hash-a", stored.BlockHash)
	assert.Equal(t, "root-a", stored.MerkleRoot)
	assert.Equal(t, mined.MerklePath.Bytes(), stored.MerklePath)
	assert.Len(t, stored.Notes, 2)
}

func TestDoSynchronizeTxStatuses_RunsStepsInOrder(t *testing.T) {
	// given: one tx that is proven and one that is not
	f := newSyncFixture(t)
	f.givenCandidates("a", "b")
	f.givenMined("a", 850_001)
	f.givenConfirmedButUnprovable("b", syncMerkleReply{err: wdk.ErrNotFoundError})

	// when:
	_, err := f.run(t.Context())

	// then:
	require.NoError(t, err)
	assert.Equal(t, []string{
		"find", "depth", "merkle:a", "mined:a", "merkle:b", "attempts", "timeouts", "review",
	}, f.rec.snapshot())
}

func TestDoSynchronizeTxStatuses_UnprovableTxsAreCountedAsFailedAttempts(t *testing.T) {
	tests := map[string]syncMerkleReply{
		"the merkle path query fails":    {err: errSyncBoom},
		"the proof is not found":         {err: wdk.ErrNotFoundError},
		"the result has no block header": {result: &wdk.MerklePathResult{MerklePath: syncMinedResult("a", 1).MerklePath}},
		"the result has no merkle path":  {result: &wdk.MerklePathResult{BlockHeader: syncMinedResult("a", 1).BlockHeader}},
	}
	for name, reply := range tests {
		t.Run(name, func(t *testing.T) {
			// given: an unprovable tx in front of a provable one
			f := newSyncFixture(t)
			f.givenCandidates("a", "b")
			f.givenConfirmedButUnprovable("a", reply)
			f.givenMined("b", 850_001)

			// when:
			statuses, err := f.run(t.Context())

			// then: the unprovable tx does not stop the run
			require.NoError(t, err)
			assert.Equal(t, []string{"b"}, syncTxIDs(statuses))

			// and: only the unprovable tx is counted as a failed attempt
			assert.Equal(t, [][]string{{"a"}}, f.knownTx.attemptBumps)
			require.Len(t, f.knownTx.mined, 1)
			assert.Equal(t, "b", f.knownTx.mined[0].TxID)
		})
	}
}

func TestDoSynchronizeTxStatuses_AbsentTxsAreCountedAsFailedAttempts(t *testing.T) {
	// given: a candidate the network does not know, next to a provable one
	f := newSyncFixture(t)
	f.givenCandidates("a", "b")
	f.givenAbsent("a")
	f.givenMined("b", 850_001)

	// when:
	statuses, err := f.run(t.Context())

	// then: the absent tx is never polled for a proof
	require.NoError(t, err)
	assert.Equal(t, []string{"b"}, syncTxIDs(statuses))
	assert.NotContains(t, f.rec.snapshot(), "merkle:a")

	// and: it is counted as a failed attempt, so its attempts can reach the timeout
	assert.Equal(t, [][]string{{"a"}}, f.knownTx.attemptBumps)

	// and: its reference and labels are looked up for the status updates that may follow
	assert.Equal(t, [][]string{{"b", "a"}}, f.txRepo.referenceLookups)
}

func TestDoSynchronizeTxStatuses_OnlyAbsentTxsStillGetAttemptsAndTimeouts(t *testing.T) {
	// given: every candidate is unknown to the network
	f := newSyncFixture(t)
	f.givenCandidates("a")
	f.givenAbsent("a")

	// when:
	statuses, err := f.run(t.Context())

	// then: the run does not stop early
	require.NoError(t, err)
	assert.Empty(t, statuses)
	assert.Equal(t, [][]string{{"a"}}, f.knownTx.attemptBumps)
	assert.Len(t, f.knownTx.timeoutArgs, 1)
}

func TestDoSynchronizeTxStatuses_AppliesProofTimeouts(t *testing.T) {
	// given: tx "c" is unknown to the network and is moved on by the proof timeout
	f := newSyncFixture(t)
	f.givenCandidates("a", "c")
	f.givenMined("a", 850_001)
	f.givenAbsent("c")
	f.knownTx.timeoutResult = []models.KnownTx{{TxID: "c", Status: wdk.ProvenTxStatusUnsent}}

	// when:
	statuses, err := f.run(t.Context())

	// then: the timeout is applied with the configured limits and the statuses used to find the txs
	require.NoError(t, err)
	require.Len(t, f.knownTx.timeoutArgs, 1)
	args := f.knownTx.timeoutArgs[0]
	assert.Equal(t, f.cfg.MaxAttempts, args.attempts)
	assert.Equal(t, f.cfg.MaxRebroadcastAttempts, args.maxRebroadcastAttempts)
	assert.Equal(t, statusesReadyToSync, args.statuses)
	assert.Equal(t, f.knownTx.findStatuses[0], args.statuses)

	// and: its updates follow the proven txs, with reference and labels but no proof
	assert.Equal(t, []wdk.TxSynchronizedStatus{
		{TxID: "a", Status: wdk.ProvenTxStatusCompleted, Reference: "ref-a", BlockHeight: 850_001, BlockHash: "hash-a", MerkleRoot: "root-a", MerklePath: f.services.merkle["a"].result.MerklePath, Labels: []string{"label-a"}},
		{TxID: "c", Status: wdk.ProvenTxStatusUnsent, Reference: "ref-c", Labels: []string{"label-c"}},
	}, statuses)
}

func TestDoSynchronizeTxStatuses_AlwaysCountsAttemptsEvenWhenNoneFailed(t *testing.T) {
	// given:
	f := newSyncFixture(t)
	f.givenCandidates("a")
	f.givenMined("a", 850_001)

	// when:
	_, err := f.run(t.Context())

	// then: the bump runs with an empty list, so the timeouts below still see current attempts
	require.NoError(t, err)
	require.Len(t, f.knownTx.attemptBumps, 1)
	assert.Empty(t, f.knownTx.attemptBumps[0])
}

func TestDoSynchronizeTxStatuses_PagesThroughCandidates(t *testing.T) {
	page := func(from, to int) []*entity.KnownTxForStatusSync {
		txs := make([]*entity.KnownTxForStatusSync, 0, to-from)
		for i := from; i < to; i++ {
			txs = append(txs, syncCandidate("tx"+strconv.Itoa(i)))
		}
		return txs
	}

	t.Run("stops at the first page that is not full", func(t *testing.T) {
		// given: two full pages and a short one
		f := newSyncFixture(t)
		f.knownTx.candidatePages = func(call int) []*entity.KnownTxForStatusSync {
			switch call {
			case 0:
				return page(0, syncTxStatusesPerPage)
			case 1:
				return page(syncTxStatusesPerPage, 2*syncTxStatusesPerPage)
			default:
				return page(2*syncTxStatusesPerPage, 2*syncTxStatusesPerPage+3)
			}
		}

		// when:
		_, err := f.run(t.Context())

		// then: all pages reach the depth filter in one call
		require.NoError(t, err)
		assert.Equal(t, 3, f.knownTx.findCalls)
		require.Len(t, f.services.statusCalls, 1)
		assert.Len(t, f.services.statusCalls[0], 2*syncTxStatusesPerPage+3)
	})

	t.Run("stops after the maximum number of pages", func(t *testing.T) {
		// given: the repo never runs out of full pages
		f := newSyncFixture(t)
		f.knownTx.candidatePages = func(call int) []*entity.KnownTxForStatusSync {
			return page(call*syncTxStatusesPerPage, (call+1)*syncTxStatusesPerPage)
		}

		// when:
		_, err := f.run(t.Context())

		// then:
		require.NoError(t, err)
		assert.Equal(t, syncTxStatusMaxPages, f.knownTx.findCalls)
		require.Len(t, f.services.statusCalls, 1)
		assert.Len(t, f.services.statusCalls[0], syncTxStatusMaxPages*syncTxStatusesPerPage)
	})
}

func TestDoSynchronizeTxStatuses_ErrorsBeforeAnyTxIsProvenReturnNothing(t *testing.T) {
	tests := map[string]struct {
		arrange func(f *syncFixture)
		message string
	}{
		"reading the last no-send check fails": {
			arrange: func(f *syncFixture) { f.kv.getErr = errSyncBoom },
			message: "failed to get statuses ready to sync",
		},
		"finding the candidates fails": {
			arrange: func(f *syncFixture) { f.knownTx.findErr = errSyncBoom },
			message: "FindKnownTxIDsReadyForStatusSync failed",
		},
		"looking up the references fails": {
			arrange: func(f *syncFixture) { f.txRepo.referencesErr = errSyncBoom },
			message: "failed to find references for txIDs",
		},
		"looking up the labels fails": {
			arrange: func(f *syncFixture) { f.txRepo.labelsErr = errSyncBoom },
			message: "failed to find labels for txIDs",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			f := newSyncFixture(t)
			f.givenCandidates("a")
			f.givenMined("a", 850_001)
			test.arrange(f)

			// when:
			statuses, err := f.run(t.Context())

			// then:
			require.ErrorIs(t, err, errSyncBoom)
			require.ErrorContains(t, err, test.message)
			assert.Nil(t, statuses)
			assert.Empty(t, f.knownTx.mined)
		})
	}
}

// Every status returned here is already committed as mined and a completed tx is never selected
// again, so dropping one would lose its event for good.
func TestDoSynchronizeTxStatuses_ErrorsAfterProvingKeepTheProvenStatuses(t *testing.T) {
	tests := map[string]struct {
		arrange     func(f *syncFixture)
		message     string
		wantTxIDs   []string
		wantMined   int
		wantBumped  bool
		wantTimeout bool
	}{
		"finding the transaction rows of the second tx fails": {
			arrange:   func(f *syncFixture) { f.txRepo.idsErrFor["b"] = errSyncBoom },
			message:   "failed to find transaction IDs by txID b",
			wantTxIDs: []string{"a"},
			wantMined: 1,
		},
		"storing the proof of the second tx fails": {
			arrange:   func(f *syncFixture) { f.knownTx.minedErrFor["b"] = errSyncBoom },
			message:   "failed to update proven txs as mined",
			wantTxIDs: []string{"a"},
			wantMined: 1,
		},
		"counting the failed attempts fails": {
			arrange:    func(f *syncFixture) { f.knownTx.attemptsErr = errSyncBoom },
			message:    "failed to increase attempts for txs",
			wantTxIDs:  []string{"a", "b"},
			wantMined:  2,
			wantBumped: true,
		},
		"applying the proof timeouts fails": {
			arrange:     func(f *syncFixture) { f.knownTx.timeoutErr = errSyncBoom },
			message:     "failed to apply proof timeouts for txs above attempts",
			wantTxIDs:   []string{"a", "b"},
			wantMined:   2,
			wantBumped:  true,
			wantTimeout: true,
		},
		"the final review fails": {
			arrange: func(f *syncFixture) {
				f.knownTx.timeoutResult = []models.KnownTx{{TxID: "c", Status: wdk.ProvenTxStatusUnsent}}
				f.knownTx.reviewErr = errSyncBoom
			},
			message:     "failed to review known tx statuses",
			wantTxIDs:   []string{"a", "b", "c"},
			wantMined:   2,
			wantBumped:  true,
			wantTimeout: true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			f := newSyncFixture(t)
			f.givenCandidates("a", "b")
			f.givenMined("a", 850_001)
			f.givenMined("b", 850_002)
			test.arrange(f)

			// when:
			statuses, err := f.run(t.Context())

			// then:
			require.ErrorIs(t, err, errSyncBoom)
			require.ErrorContains(t, err, test.message)
			assert.Equal(t, test.wantTxIDs, syncTxIDs(statuses))
			assert.Len(t, f.knownTx.mined, test.wantMined)
			assert.Equal(t, test.wantBumped, len(f.knownTx.attemptBumps) > 0)
			assert.Equal(t, test.wantTimeout, len(f.knownTx.timeoutArgs) > 0)
		})
	}
}

func TestDoSynchronizeTxStatuses_StopsWhenContextIsCanceled(t *testing.T) {
	// given: three provable txs and a context that is canceled while the first proof is fetched
	f := newSyncFixture(t)
	f.givenCandidates("a", "b", "c")
	f.givenMined("a", 850_001)
	f.givenMined("b", 850_002)
	f.givenMined("c", 850_003)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	f.services.onMerklePath = func(txID string) {
		if txID == "a" {
			cancel()
		}
	}

	// when:
	statuses, err := f.run(ctx)

	// then: the tx being proven when the cancel hit is finished, the rest is not started
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorContains(t, err, "context canceled, aborting synchronizeTxStatuses")
	assert.Equal(t, []string{"a"}, syncTxIDs(statuses))
	assert.NotContains(t, f.rec.snapshot(), "merkle:b")

	// and: nothing after the loop runs
	assert.Empty(t, f.knownTx.attemptBumps)
	assert.Empty(t, f.knownTx.timeoutArgs)
	assert.Equal(t, 0, f.knownTx.reviewCalls)
}

func TestGetStatusesReadyToSync(t *testing.T) {
	t.Run("includes no-send when it was never checked and records the check", func(t *testing.T) {
		// given:
		f := newSyncFixture(t)
		delete(f.kv.data, noSendLastCheck)

		// when:
		statuses, err := f.sync.getStatusesReadyToSync(t.Context())

		// then:
		require.NoError(t, err)
		assert.Equal(t, append(append([]wdk.ProvenTxReqStatus{}, statusesReadyToSync...), wdk.ProvenTxStatusNoSend), statuses)
		assert.Contains(t, f.kv.data, noSendLastCheck)
	})

	t.Run("includes no-send once the check period has passed", func(t *testing.T) {
		// given:
		f := newSyncFixture(t)
		f.kv.data[noSendLastCheck] = []byte(time.Now().Add(-25 * time.Hour).Format(time.RFC3339))

		// when:
		statuses, err := f.sync.getStatusesReadyToSync(t.Context())

		// then:
		require.NoError(t, err)
		assert.Contains(t, statuses, wdk.ProvenTxStatusNoSend)
	})

	t.Run("leaves no-send out inside the check period", func(t *testing.T) {
		// given:
		f := newSyncFixture(t)

		// when:
		statuses, err := f.sync.getStatusesReadyToSync(t.Context())

		// then:
		require.NoError(t, err)
		assert.Equal(t, statusesReadyToSync, statuses)
	})

	t.Run("leaves no-send out and resets the check when the stored time is unreadable", func(t *testing.T) {
		// given:
		f := newSyncFixture(t)
		f.kv.data[noSendLastCheck] = []byte("not a time")

		// when:
		statuses, err := f.sync.getStatusesReadyToSync(t.Context())

		// then:
		require.NoError(t, err)
		assert.Equal(t, statusesReadyToSync, statuses)
		_, parseErr := time.Parse(time.RFC3339, string(f.kv.data[noSendLastCheck]))
		assert.NoError(t, parseErr)
	})
}

func TestReviewKnownTxStatuses(t *testing.T) {
	failedTxs := func(txIDs ...string) []*entity.KnownTxForStatusSync {
		txs := make([]*entity.KnownTxForStatusSync, 0, len(txIDs))
		for _, txID := range txIDs {
			txs = append(txs, &entity.KnownTxForStatusSync{TxID: txID, Status: wdk.ProvenTxStatusInvalid})
		}
		return txs
	}

	t.Run("fails the transactions of terminally failed txs and loses their inputs", func(t *testing.T) {
		// given: one failed tx with two transaction rows and one with a single row
		f := newSyncFixture(t)
		f.txRepo.ids["a"] = []uint{1, 2}
		f.txRepo.ids["b"] = []uint{3}
		f.knownTx.reviewPages = func(call int) []*entity.KnownTxForStatusSync {
			if call == 0 {
				return failedTxs("a", "b")
			}
			return nil
		}

		// when:
		err := f.sync.reviewKnownTxStatuses(t.Context())

		// then: every transaction row is failed and the outputs it created are not spendable
		require.NoError(t, err)
		assert.Equal(t, []uint{1, 2, 3}, f.txRepo.failedTransactionIDs)
		assert.Equal(t, []uint{1, 2, 3}, f.outputs.notSpendable)

		// and: the inputs it spent are NOT restored: the verdict can be a false positive
		assert.Empty(t, f.outputs.restored)
	})

	t.Run("asks for the terminal failure statuses a page at a time", func(t *testing.T) {
		// given:
		f := newSyncFixture(t)

		// when:
		err := f.sync.reviewKnownTxStatuses(t.Context())

		// then:
		require.NoError(t, err)
		assert.Equal(t, [][]wdk.ProvenTxReqStatus{{wdk.ProvenTxStatusInvalid, wdk.ProvenTxStatusDoubleSpend}}, f.knownTx.reviewStatus)
		assert.Equal(t, []int{reviewKnownTxStatusesPerPage}, f.knownTx.reviewLimits)
	})

	t.Run("reads the next page only after a full one", func(t *testing.T) {
		// given: a full page followed by a short one
		f := newSyncFixture(t)
		f.knownTx.reviewPages = func(call int) []*entity.KnownTxForStatusSync {
			size := reviewKnownTxStatusesPerPage
			if call > 0 {
				size = 1
			}
			txs := make([]*entity.KnownTxForStatusSync, 0, size)
			for i := range size {
				txs = append(txs, &entity.KnownTxForStatusSync{TxID: fmt.Sprintf("tx-%d-%d", call, i)})
			}
			return txs
		}

		// when:
		err := f.sync.reviewKnownTxStatuses(t.Context())

		// then:
		require.NoError(t, err)
		assert.Equal(t, 2, f.knownTx.reviewCalls)
	})

	t.Run("stops after the maximum number of pages", func(t *testing.T) {
		// given: the repo never runs out of full pages
		f := newSyncFixture(t)
		f.knownTx.reviewPages = func(call int) []*entity.KnownTxForStatusSync {
			txs := make([]*entity.KnownTxForStatusSync, 0, reviewKnownTxStatusesPerPage)
			for i := range reviewKnownTxStatusesPerPage {
				txs = append(txs, &entity.KnownTxForStatusSync{TxID: fmt.Sprintf("tx-%d-%d", call, i)})
			}
			return txs
		}

		// when:
		err := f.sync.reviewKnownTxStatuses(t.Context())

		// then:
		require.NoError(t, err)
		assert.Equal(t, reviewKnownTxStatusesMaxPages, f.knownTx.reviewCalls)
	})

	t.Run("returns the error of the step that fails", func(t *testing.T) {
		tests := map[string]struct {
			arrange func(f *syncFixture)
			message string
		}{
			"finding the failed txs": {
				arrange: func(f *syncFixture) { f.knownTx.reviewErr = errSyncBoom },
				message: "failed to find terminal failed known txs needing review",
			},
			"finding the transaction rows": {
				arrange: func(f *syncFixture) { f.txRepo.idsErrFor["a"] = errSyncBoom },
				message: "failed to find transaction IDs for terminal failed tx a",
			},
			"failing the transaction": {
				arrange: func(f *syncFixture) { f.txRepo.statusUpdateErr = errSyncBoom },
				message: "failed to set failed transaction status for tx a",
			},
			"marking the created outputs": {
				arrange: func(f *syncFixture) { f.outputs.markErr = errSyncBoom },
				message: "failed to mark created outputs as not spendable for terminal failed tx a",
			},
			"running the unit of work": {
				arrange: func(f *syncFixture) { f.uow.err = errSyncBoom },
				message: "boom",
			},
		}
		for name, test := range tests {
			t.Run(name, func(t *testing.T) {
				// given:
				f := newSyncFixture(t)
				f.txRepo.ids["a"] = []uint{1}
				f.knownTx.reviewPages = func(int) []*entity.KnownTxForStatusSync { return failedTxs("a") }
				test.arrange(f)

				// when:
				err := f.sync.reviewKnownTxStatuses(t.Context())

				// then:
				require.ErrorIs(t, err, errSyncBoom)
				require.ErrorContains(t, err, test.message)
			})
		}
	})
}
