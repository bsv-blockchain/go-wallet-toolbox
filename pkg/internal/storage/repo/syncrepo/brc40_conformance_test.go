package syncrepo_test

import (
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	conformancevectors "github.com/bsv-blockchain/go-wallet-toolbox/conformance/vectors/sync"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const defaultBasket = "default"

// BRC-40 conformance vector runner.
//
// Consumes the vendored conformance corpus embedded via go:embed in:
//   conformance/vectors/sync/sync.go
//
// Origin: bsv-blockchain/ts-stack, pinned via conformance/SOURCE.
// Refresh: ./conformance/scripts/refresh-vectors.sh
//
// Reference dispatcher contract:
//   ts-stack/conformance/runner/ts/dispatchers/sync.ts
//
// For brc40/mergeExisting vectors the runner seeds the `existing` row into the
// Go syncrepo, calls the upsert with `incoming`, then compares the resulting
// post-state to expected.action (update vs skip).
//
// For brc40/flow vectors with `messages[]` + `expected.finalState`, the runner
// replays each syncChunk against the syncrepo and asserts the post-merge state
// per natural key matches the finalState block.
//
// $BRC40_VECTORS_FILE may override the embedded corpus for ad-hoc testing
// against an unreleased upstream version.

type brc40Vector struct {
	ID          string        `json:"id"`
	Description string        `json:"description"`
	Input       brc40Input    `json:"input"`
	Expected    brc40Expected `json:"expected"`
	Tags        []string      `json:"tags"`
	Skip        bool          `json:"skip"`
}

type brc40Input struct {
	Channel  string           `json:"channel"`
	Entity   string           `json:"entity"`
	Existing map[string]any   `json:"existing"`
	Incoming map[string]any   `json:"incoming"`
	Messages []map[string]any `json:"messages"`
	Message  map[string]any   `json:"message"`
	Request  map[string]any   `json:"request"`
	Response map[string]any   `json:"response"`
}

type brc40Expected struct {
	Valid      bool           `json:"valid"`
	Action     string         `json:"action"`
	FinalState map[string]any `json:"finalState"`
	Done       bool           `json:"done"`
}

type brc40File struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Vectors []brc40Vector `json:"vectors"`
}

func loadBRC40Vectors(t *testing.T) brc40File {
	t.Helper()
	data := conformancevectors.BRC40UserState
	if p := os.Getenv("BRC40_VECTORS_FILE"); p != "" {
		// #nosec G304 -- developer-supplied override path for ad-hoc upstream
		// testing; only consulted when the env var is explicitly set in a dev
		// shell, never in CI.
		override, err := os.ReadFile(p)
		require.NoError(t, err, "BRC40_VECTORS_FILE=%s unreadable", p)
		data = override
	}
	require.NotEmpty(t, data, "embedded BRC-40 vectors empty — refresh via ./conformance/scripts/refresh-vectors.sh")
	var f brc40File
	require.NoError(t, json.Unmarshal(data, &f))
	return f
}

func parseISO(t *testing.T, s string) time.Time {
	t.Helper()
	tt, err := time.Parse(time.RFC3339Nano, s)
	require.NoError(t, err)
	return tt
}

// TestBRC40Conformance_MergeExisting iterates each brc40/mergeExisting vector,
// drives the Go syncrepo upserts, and asserts the merge action matches
// expected.action ("update" or "skip"). Covers regression set from
// go-wallet-toolbox#853:
//   - sync.brc40.merge.tx.1
//   - sync.brc40.merge.tx.error.regression.{1,2}
//   - sync.brc40.merge.output.error.regression.{1,2}
//   - sync.brc40.merge.proventx.error.regression.1
func TestBRC40Conformance_MergeExisting(t *testing.T) {
	f := loadBRC40Vectors(t)

	for _, v := range f.Vectors {
		if v.Skip || v.Input.Channel != "brc40/mergeExisting" {
			continue
		}
		t.Run(v.ID, func(t *testing.T) {
			switch v.Input.Entity {
			case "transactions":
				runMergeExistingTransaction(t, v)
			case "outputs":
				runMergeExistingOutput(t, v)
			case "provenTxs":
				runMergeExistingProvenTx(t, v)
			default:
				t.Skipf("unsupported entity %q", v.Input.Entity)
			}
		})
	}
}

func runMergeExistingTransaction(t *testing.T, v brc40Vector) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	user, err := repos.CreateUser(
		t.Context(), testusers.Alice.IdentityKey(t), "test-storage",
		wdk.BasketConfiguration{Name: defaultBasket, NumberOfDesiredUTXOs: 1, MinimumDesiredUTXOValue: 1000},
	)
	require.NoError(t, err)

	existing := v.Input.Existing
	incoming := v.Input.Incoming

	// Use vector's transactionId encoded into a stable reference per row.
	reference := "vector-tx-" + asString(existing["transactionId"])

	// Map vector.provenTxId → Go Transaction.TxID. The regression vectors pin
	// that a stale incoming with provenTxId=null MUST NOT clear the existing
	// row's proven-tx pointer; mirroring that in Go means TxID must not flip
	// from non-nil to nil.
	existingStatus := wdk.TxStatus(asString(existing["status"]))
	existingTxID := provenTxIDToTxIDPtr(existing["provenTxId"], "existing")
	_, _, err = repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt:  parseISO(t, asString(existing["created_at"])),
		UpdatedAt:  parseISO(t, asString(existing["updated_at"])),
		UserID:     user.ID,
		Status:     existingStatus,
		Reference:  reference,
		IsOutgoing: false,
		Satoshis:   1,
		TxID:       existingTxID,
	})
	require.NoError(t, err)

	incomingStatus := wdk.TxStatus(asString(incoming["status"]))
	incomingTxID := provenTxIDToTxIDPtr(incoming["provenTxId"], "incoming")
	_, _, err = repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt:  parseISO(t, asString(incoming["created_at"])),
		UpdatedAt:  parseISO(t, asString(incoming["updated_at"])),
		UserID:     user.ID,
		Status:     incomingStatus,
		Reference:  reference,
		IsOutgoing: false,
		Satoshis:   1,
		TxID:       incomingTxID,
	})
	require.NoError(t, err)

	var got models.Transaction
	require.NoError(t, d.DB.Where("user_id = ? AND reference = ?", user.ID, reference).First(&got).Error)
	assertMergeAction(t, v, gotTxFields(got, existingStatus, incomingStatus, existingTxID, incomingTxID))
}

func runMergeExistingOutput(t *testing.T, v brc40Vector) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	user, err := repos.CreateUser(
		t.Context(), testusers.Alice.IdentityKey(t), "test-storage",
		wdk.BasketConfiguration{Name: defaultBasket, NumberOfDesiredUTXOs: 1, MinimumDesiredUTXOValue: 1000},
	)
	require.NoError(t, err)

	// Seed the parent transaction (status=completed so output UTXO path is exercised).
	reference := "vector-out-tx-" + asString(v.Input.Existing["transactionId"])
	txID := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	_, txnDBID, err := repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt:  parseISO(t, asString(v.Input.Existing["created_at"])),
		UpdatedAt:  parseISO(t, asString(v.Input.Existing["updated_at"])),
		UserID:     user.ID,
		Status:     wdk.TxStatusCompleted,
		Reference:  reference,
		IsOutgoing: true,
		Satoshis:   asInt64(v.Input.Existing["satoshis"]),
		TxID:       &txID,
	})
	require.NoError(t, err)

	basket := defaultBasket
	existingSpentBy := asOptionalUint(t, v.Input.Existing["spentBy"])
	_, outID, err := repos.UpsertOutputForSync(t.Context(), &entity.Output{
		CreatedAt:     parseISO(t, asString(v.Input.Existing["created_at"])),
		UpdatedAt:     parseISO(t, asString(v.Input.Existing["updated_at"])),
		UserID:        user.ID,
		TransactionID: txnDBID,
		SpentBy:       existingSpentBy,
		Satoshis:      asInt64(v.Input.Existing["satoshis"]),
		Vout:          asUint32(t, v.Input.Existing["vout"]),
		BasketName:    &basket,
		Spendable:     asBool(v.Input.Existing["spendable"]),
		Description:   "existing",
	})
	require.NoError(t, err)

	// Drive incoming.
	incomingSpentBy := asOptionalUint(t, v.Input.Incoming["spentBy"])
	_, _, err = repos.UpsertOutputForSync(t.Context(), &entity.Output{
		CreatedAt:     parseISO(t, asString(v.Input.Incoming["created_at"])),
		UpdatedAt:     parseISO(t, asString(v.Input.Incoming["updated_at"])),
		UserID:        user.ID,
		TransactionID: txnDBID,
		SpentBy:       incomingSpentBy,
		Satoshis:      asInt64(v.Input.Incoming["satoshis"]),
		Vout:          asUint32(t, v.Input.Incoming["vout"]),
		BasketName:    &basket,
		Spendable:     asBool(v.Input.Incoming["spendable"]),
		Description:   "incoming",
	})
	require.NoError(t, err)

	var got models.Output
	require.NoError(t, d.DB.First(&got, outID).Error)

	expectUpdate := v.Expected.Action == "update"
	if expectUpdate {
		require.Equal(t, "incoming", got.Description, "vector %s expects update", v.ID)
		require.Equal(t, asBool(v.Input.Incoming["spendable"]), got.Spendable)
		assertOptionalUintEqual(t, incomingSpentBy, got.SpentBy)
	} else {
		require.Equal(t, "existing", got.Description, "vector %s expects skip", v.ID)
		require.Equal(t, asBool(v.Input.Existing["spendable"]), got.Spendable,
			"stale chunk MUST NOT regress spendable")
		assertOptionalUintEqual(t, existingSpentBy, got.SpentBy)
	}
}

func runMergeExistingProvenTx(t *testing.T, v brc40Vector) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	existing := v.Input.Existing
	incoming := v.Input.Incoming
	txid := asString(existing["txid"])

	existingMerkle := []byte(asString(existing["merklePath"]))
	existingHeight := asUint32(t, existing["height"])
	existingRoot := "root-existing"
	existingHash := "hash-existing"
	_, err := repos.UpsertKnownTxForSync(t.Context(), &entity.KnownTx{
		CreatedAt:   parseISO(t, asString(existing["created_at"])),
		UpdatedAt:   parseISO(t, asString(existing["updated_at"])),
		TxID:        txid,
		Status:      wdk.ProvenTxStatusCompleted,
		MerklePath:  existingMerkle,
		MerkleRoot:  &existingRoot,
		BlockHash:   &existingHash,
		BlockHeight: &existingHeight,
	})
	require.NoError(t, err)

	incomingMerkle := []byte(asString(incoming["merklePath"]))
	incomingHeight := asUint32(t, incoming["height"])
	incomingRoot := "root-incoming"
	incomingHash := "hash-incoming"
	_, err = repos.UpsertKnownTxForSync(t.Context(), &entity.KnownTx{
		CreatedAt:   parseISO(t, asString(incoming["created_at"])),
		UpdatedAt:   parseISO(t, asString(incoming["updated_at"])),
		TxID:        txid,
		Status:      wdk.ProvenTxStatusCompleted,
		MerklePath:  incomingMerkle,
		MerkleRoot:  &incomingRoot,
		BlockHash:   &incomingHash,
		BlockHeight: &incomingHeight,
	})
	require.NoError(t, err)

	var got models.KnownTx
	require.NoError(t, d.DB.First(&got, "tx_id = ?", txid).Error)

	if v.Expected.Action == "update" {
		require.NotNil(t, got.MerkleRoot)
		require.Equal(t, incomingRoot, *got.MerkleRoot, "vector %s expects update", v.ID)
	} else {
		require.NotNil(t, got.MerkleRoot)
		require.Equal(t, existingRoot, *got.MerkleRoot,
			"stale chunk MUST NOT overwrite proven tx (vector %s)", v.ID)
	}
}

type txAssertCtx struct {
	got            models.Transaction
	existingStatus wdk.TxStatus
	incomingStatus wdk.TxStatus
	existingTxID   *string
	incomingTxID   *string
}

func gotTxFields(got models.Transaction, e, i wdk.TxStatus, eTxID, iTxID *string) txAssertCtx {
	return txAssertCtx{got: got, existingStatus: e, incomingStatus: i, existingTxID: eTxID, incomingTxID: iTxID}
}

func assertMergeAction(t *testing.T, v brc40Vector, ctx txAssertCtx) {
	t.Helper()
	if v.Expected.Action == "update" {
		require.Equal(t, ctx.incomingStatus, ctx.got.Status, "vector %s expects update", v.ID)
		assertOptionalStringEqual(t, ctx.incomingTxID, ctx.got.TxID)
	} else {
		require.Equal(t, ctx.existingStatus, ctx.got.Status,
			"stale chunk MUST NOT regress status (vector %s)", v.ID)
		assertOptionalStringEqual(t, ctx.existingTxID, ctx.got.TxID)
	}
}

// TestBRC40Conformance_FlowRegression iterates each brc40/flow vector with
// `messages[]` + `expected.finalState`, replays each syncChunk against syncrepo,
// then asserts the final per-natural-key state matches expected.finalState.
// Pins sync.brc40.flow.regression.1 (the two-chunk replay from #853).
func TestBRC40Conformance_FlowRegression(t *testing.T) {
	f := loadBRC40Vectors(t)
	for _, v := range f.Vectors {
		if v.Skip || v.Input.Channel != "brc40/flow" || v.Expected.FinalState == nil {
			continue
		}
		t.Run(v.ID, func(t *testing.T) {
			runFlowReplay(t, v)
		})
	}
}

func runFlowReplay(t *testing.T, v brc40Vector) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	user, err := repos.CreateUser(
		t.Context(), testusers.Alice.IdentityKey(t), "test-storage",
		wdk.BasketConfiguration{Name: defaultBasket, NumberOfDesiredUTXOs: 1, MinimumDesiredUTXOValue: 1000},
	)
	require.NoError(t, err)

	// Replay messages in order.
	for _, msg := range v.Input.Messages {
		sc := asMap(msg["syncChunk"])
		if txs, ok := sc["transactions"].([]any); ok {
			for _, r := range txs {
				row := asMap(r)
				replayTransactionChunk(t, repos, user.ID, row)
			}
		}
		// (other entity arrays can be added here as further vectors require)
	}

	// Verify finalState per entity.
	if txs, ok := v.Expected.FinalState["transactions"].([]any); ok {
		for _, r := range txs {
			row := asMap(r)
			ref := "vector-flow-tx-" + asString(row["transactionId"])
			var got models.Transaction
			require.NoError(t, d.DB.
				Where("user_id = ? AND reference = ?", user.ID, ref).
				First(&got).Error)
			require.Equal(t, wdk.TxStatus(asString(row["status"])), got.Status,
				"vector %s: final status for transactionId=%v", v.ID, row["transactionId"])
		}
	}
}

func replayTransactionChunk(t *testing.T, repos *repo.Repositories, userID int, row map[string]any) {
	t.Helper()
	reference := "vector-flow-tx-" + asString(row["transactionId"])
	status := wdk.TxStatus(asString(row["status"]))
	_, _, err := repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt:  parseISO(t, asString(row["created_at"])),
		UpdatedAt:  parseISO(t, asString(row["updated_at"])),
		UserID:     userID,
		Status:     status,
		Reference:  reference,
		IsOutgoing: false,
		Satoshis:   1,
	})
	require.NoError(t, err)
}

// ── small json helpers ────────────────────────────────────────────────────────

func asString(v any) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

func asInt64(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	}
	return 0
}

func asOptionalUint(t *testing.T, v any) *uint {
	t.Helper()
	if v == nil {
		return nil
	}
	n := asInt64(v)
	require.GreaterOrEqual(t, n, int64(0), "negative value %d cannot fit in uint", n)
	u := uint(n) //nolint:gosec // bounded by GreaterOrEqual check above
	return &u
}

func asUint32(t *testing.T, v any) uint32 {
	t.Helper()
	n := asInt64(v)
	require.GreaterOrEqual(t, n, int64(0), "negative value %d cannot fit in uint32", n)
	require.LessOrEqual(t, n, int64(math.MaxUint32), "value %d overflows uint32", n)
	return uint32(n) //nolint:gosec // bounded by LessOrEqual check above
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// provenTxIDToTxIDPtr maps a vector's provenTxId field (number or null) to a
// Go Transaction.TxID pointer. A null/missing provenTxId becomes nil; any
// non-null value becomes a stable synthesized string. The skip regression
// vectors pin that a stale chunk with provenTxId=null MUST NOT clear an
// existing row's TxID.
//
// JSON-encoded for stable formatting across number / string variants the
// upstream vectors may use for provenTxId.
func provenTxIDToTxIDPtr(provenTxID any, tag string) *string {
	if provenTxID == nil {
		return nil
	}
	b, err := json.Marshal(provenTxID)
	if err != nil {
		// JSON-decoded any is always re-encodable; this is unreachable.
		panic("provenTxIDToTxIDPtr: json.Marshal failed: " + err.Error())
	}
	s := "ptx-" + string(b) + "-" + tag
	return &s
}

func assertOptionalStringEqual(t *testing.T, want, got *string) {
	t.Helper()
	if want == nil {
		require.Nil(t, got)
		return
	}
	require.NotNil(t, got)
	require.Equal(t, *want, *got)
}

func assertOptionalUintEqual(t *testing.T, want, got *uint) {
	t.Helper()
	if want == nil {
		require.Nil(t, got)
		return
	}
	require.NotNil(t, got)
	require.Equal(t, *want, *got)
}
