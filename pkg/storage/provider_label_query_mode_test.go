package storage_test

import (
	"testing"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// Label and tag filters in "all" mode must match exactly the rows that carry
// every requested name - for this user, ignoring soft-deleted associations - and
// report a total that ignores paging. The cases cover what a query rewrite could
// get wrong: order, repeats, a name nobody has, a name another user has, and a
// label or tag that was removed.

func TestListActions_LabelQueryModes(t *testing.T) {
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	create := func(i int, labels ...string) string {
		_, tx := given.Action(activeStorage).
			WithSatoshisToInternalize(uint64(50_000 + i*1_000)). //nolint:gosec // small test index
			WithSatoshisToSend(uint64(1_000 + i)).               //nolint:gosec // small test index
			WithLabels(labels...).
			Processed()
		return tx.TxID().String()
	}

	a := create(0, "lbl-x", "lbl-y")
	b := create(1, "lbl-x")
	c := create(2, "lbl-y")
	d := create(3, "lbl-x", "lbl-y", "lbl-z")
	e := create(4, "lbl-x", "lbl-y")

	// b carries lbl-bob, but the association belongs to another user: a label is
	// matched only within the querying user's own labels
	attachForeignLabel(t, activeStorage, b, "lbl-bob", testusers.Bob.ID)

	// e loses lbl-y: a soft-deleted association must not count
	softDeleteLabel(t, activeStorage, e, "lbl-y")

	cases := map[string]struct {
		mode   defs.QueryMode
		labels []string
		want   []string
	}{
		"all: one label":                  {defs.QueryModeAll, []string{"lbl-x"}, []string{a, b, d, e}},
		"all: two labels":                 {defs.QueryModeAll, []string{"lbl-x", "lbl-y"}, []string{a, d}},
		"all: order does not matter":      {defs.QueryModeAll, []string{"lbl-y", "lbl-x"}, []string{a, d}},
		"all: repeated label":             {defs.QueryModeAll, []string{"lbl-x", "lbl-y", "lbl-y"}, []string{a, d}},
		"all: three labels":               {defs.QueryModeAll, []string{"lbl-x", "lbl-y", "lbl-z"}, []string{d}},
		"all: unknown label excludes all": {defs.QueryModeAll, []string{"lbl-x", "lbl-nope"}, nil},
		"all: other user's label only":    {defs.QueryModeAll, []string{"lbl-x", "lbl-bob"}, nil},
		"all: soft-deleted label":         {defs.QueryModeAll, []string{"lbl-y"}, []string{a, c, d}},
		"any: two labels":                 {defs.QueryModeAny, []string{"lbl-x", "lbl-y"}, []string{a, b, c, d, e}},
		"any: known and unknown":          {defs.QueryModeAny, []string{"lbl-z", "lbl-nope"}, []string{d}},
		"any: other user's label only":    {defs.QueryModeAny, []string{"lbl-bob"}, nil},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for _, includeOutputs := range []bool{false, true} {
				args := wdk.ListActionsArgs{
					Labels:         toLabels(tc.labels),
					LabelQueryMode: to.Ptr(tc.mode),
					IncludeLabels:  to.Ptr(primitives.BooleanDefaultFalse(true)),
					IncludeOutputs: to.Ptr(primitives.BooleanDefaultFalse(includeOutputs)),
					Limit:          100,
				}

				result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), args)
				require.NoError(t, err)
				assert.ElementsMatch(t, tc.want, actionTxIDs(result), "includeOutputs=%t", includeOutputs)
				assert.EqualValues(t, len(tc.want), result.TotalActions, "includeOutputs=%t", includeOutputs)

				if len(tc.want) > 1 {
					// the total ignores paging
					args.Limit = 1
					paged, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), args)
					require.NoError(t, err)
					assert.Len(t, paged.Actions, 1, "includeOutputs=%t", includeOutputs)
					assert.EqualValues(t, len(tc.want), paged.TotalActions, "includeOutputs=%t", includeOutputs)
				}
			}
		})
	}
}

func TestListOutputs_TagQueryModes(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	aliceFaucet := given.Faucet(activeStorage, testusers.Alice)
	aliceFaucet.TopUp(1000) // tags: create-action tag, faucet-0
	aliceFaucet.TopUp(1001) // tags: create-action tag, faucet-1
	aliceFaucet.TopUp(1002) // tags: create-action tag, faucet-2
	given.Faucet(activeStorage, testusers.Bob).TopUp(2000)

	all := listTaggedOutputs(t, activeStorage, defs.QueryModeAny, "faucet-0", "faucet-1", "faucet-2")
	require.Len(t, all, 3, "precondition: one output per faucet tag")
	common := fixtures.CreateActionTestTag

	// the faucet-2 output loses the common tag
	softDeleteTag(t, activeStorage, "faucet-2", common)

	cases := map[string]struct {
		mode defs.QueryMode
		tags []string
		want []string
	}{
		"all: common tag":               {defs.QueryModeAll, []string{common}, []string{"faucet-0", "faucet-1"}},
		"all: common and specific":      {defs.QueryModeAll, []string{common, "faucet-1"}, []string{"faucet-1"}},
		"all: order does not matter":    {defs.QueryModeAll, []string{"faucet-1", common}, []string{"faucet-1"}},
		"all: two specific tags":        {defs.QueryModeAll, []string{"faucet-0", "faucet-1"}, nil},
		"all: soft-deleted tag":         {defs.QueryModeAll, []string{common, "faucet-2"}, nil},
		"all: unknown tag excludes all": {defs.QueryModeAll, []string{common, "tag-nope"}, nil},
		"any: two specific tags":        {defs.QueryModeAny, []string{"faucet-0", "faucet-1"}, []string{"faucet-0", "faucet-1"}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := listTaggedOutputs(t, activeStorage, tc.mode, tc.tags...)
			want := make([]string, 0, len(tc.want))
			for _, faucetTag := range tc.want {
				want = append(want, all[faucetTag])
			}
			gotOutpoints := make([]string, 0, len(got))
			for _, outpoint := range got {
				gotOutpoints = append(gotOutpoints, outpoint)
			}
			assert.ElementsMatch(t, want, gotOutpoints)
		})
	}
}

func toLabels(names []string) []primitives.StringUnder300 {
	out := make([]primitives.StringUnder300, 0, len(names))
	for _, n := range names {
		out = append(out, primitives.StringUnder300(n))
	}
	return out
}

func softDeleteLabel(t *testing.T, activeStorage *storage.Provider, txID, label string) {
	t.Helper()
	var ids []uint
	require.NoError(t, activeStorage.Database.DB.Model(&models.Transaction{}).Where("tx_id = ?", txID).Pluck("id", &ids).Error)
	require.Len(t, ids, 1)
	res := activeStorage.Database.DB.Where("transaction_id = ? AND label_name = ?", ids[0], label).Delete(&models.TransactionLabel{})
	require.NoError(t, res.Error)
	require.EqualValues(t, 1, res.RowsAffected)
}

// attachForeignLabel labels txID with a label owned by another user.
func attachForeignLabel(t *testing.T, activeStorage *storage.Provider, txID, label string, userID int) {
	t.Helper()
	db := activeStorage.Database.DB
	var ids []uint
	require.NoError(t, db.Model(&models.Transaction{}).Where("tx_id = ?", txID).Pluck("id", &ids).Error)
	require.Len(t, ids, 1)
	require.NoError(t, db.Create(&models.Label{Name: label, UserID: userID}).Error)
	require.NoError(t, db.Create(&models.TransactionLabel{TransactionID: ids[0], LabelName: label, LabelUserID: userID}).Error)
}

// softDeleteTag removes tag from the output that carries identifyingTag.
func softDeleteTag(t *testing.T, activeStorage *storage.Provider, identifyingTag, tag string) {
	t.Helper()
	var outputIDs []uint
	require.NoError(t, activeStorage.Database.DB.Model(&models.OutputTag{}).
		Where("tag_name = ?", identifyingTag).
		Pluck("output_id", &outputIDs).Error)
	require.Len(t, outputIDs, 1)
	res := activeStorage.Database.DB.Where("output_id = ? AND tag_name = ?", outputIDs[0], tag).Delete(&models.OutputTag{})
	require.NoError(t, res.Error)
	require.EqualValues(t, 1, res.RowsAffected)
}

// listTaggedOutputs returns Alice's outputs matching the tags, keyed by their
// faucet-N tag.
func listTaggedOutputs(t *testing.T, activeStorage *storage.Provider, mode defs.QueryMode, tags ...string) map[string]string {
	t.Helper()
	result, err := activeStorage.ListOutputs(t.Context(), testusers.Alice.AuthID(), wdk.ListOutputsArgs{
		Limit:        100,
		IncludeTags:  true,
		Tags:         toLabels(tags),
		TagQueryMode: to.Ptr(mode),
	})
	require.NoError(t, err)
	assert.EqualValues(t, len(result.Outputs), result.TotalOutputs)

	byFaucet := make(map[string]string, len(result.Outputs))
	for _, o := range result.Outputs {
		for _, tag := range o.Tags {
			if len(tag) > len("faucet-") && string(tag)[:len("faucet-")] == "faucet-" {
				byFaucet[string(tag)] = string(o.Outpoint)
			}
		}
	}
	return byFaucet
}
