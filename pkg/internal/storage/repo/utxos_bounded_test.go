package repo_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// boundedUTXOsFixture seeds one user with a fixed, read-only UTXO dataset used by
// every subtest of the bounded-query tests (the queries under test never mutate):
//
//	mined:    100(A), 200(B), 300(C), 1000(D), 1000(E), 1200(F), 5000(G)
//	unproven: 150(H), 1000(I), 4000(J)
//	sending:  250(K)
//	mined, reserved: 999(L) — must never be returned
type boundedUTXOsFixture struct {
	repos  *repo.UTXOs
	db     *gorm.DB
	userID int

	a, b, c, d, e, f, g uint // mined
	h, i, j             uint // unproven
	k                   uint // sending
	l                   uint // mined but reserved
}

func newBoundedUTXOsFixture(t *testing.T) (*boundedUTXOsFixture, func()) {
	t.Helper()

	db, cleanup := dbfixtures.TestDatabase(t)
	repos := repo.NewSQLRepositories(db.DB)
	ctx := t.Context()

	user, err := repos.CreateUser(ctx, "bounded-utxos-user", "test-storage", wdk.DefaultBasketConfiguration())
	require.NoError(t, err)

	ownerTx := createTestTx(t, repos, ctx, user.ID, "bounded-utxos-owner-tx")
	reservingTx := createTestTx(t, repos, ctx, user.ID, "bounded-utxos-reserving-tx")

	var vout uint32
	seed := func(sats uint64, status wdk.UTXOStatus, reservedBy *uint) uint {
		output := createTestOutput(t, db.DB, user.ID, ownerTx.ID, vout)
		vout++

		utxo := models.UserUTXO{
			UserID:             user.ID,
			OutputID:           output.ID,
			BasketName:         wdk.BasketNameForChange,
			Satoshis:           sats,
			EstimatedInputSize: 148,
			UTXOStatus:         status,
			ReservedByID:       reservedBy,
		}
		require.NoError(t, db.DB.Create(&utxo).Error)
		return output.ID
	}

	fx := &boundedUTXOsFixture{repos: repos.UTXOs, db: db.DB}
	fx.a = seed(100, wdk.UTXOStatusMined, nil)
	fx.b = seed(200, wdk.UTXOStatusMined, nil)
	fx.c = seed(300, wdk.UTXOStatusMined, nil)
	fx.d = seed(1000, wdk.UTXOStatusMined, nil)
	fx.e = seed(1000, wdk.UTXOStatusMined, nil)
	fx.f = seed(1200, wdk.UTXOStatusMined, nil)
	fx.g = seed(5000, wdk.UTXOStatusMined, nil)
	fx.h = seed(150, wdk.UTXOStatusUnproven, nil)
	fx.i = seed(1000, wdk.UTXOStatusUnproven, nil)
	fx.j = seed(4000, wdk.UTXOStatusUnproven, nil)
	fx.k = seed(250, wdk.UTXOStatusSending, nil)
	fx.l = seed(999, wdk.UTXOStatusMined, &reservingTx.ID)

	fx.userID = user.ID
	return fx, cleanup
}

func (fx *boundedUTXOsFixture) sufficient(t *testing.T, status wdk.UTXOStatus, minSatoshis uint64, excluded []uint) *models.UserUTXO {
	t.Helper()
	result, err := fx.repos.FindSmallestSufficientUTXOForUpdate(t.Context(), fx.db, fx.userID, wdk.BasketNameForChange, status, minSatoshis, excluded)
	require.NoError(t, err)
	return result
}

func (fx *boundedUTXOsFixture) insufficient(t *testing.T, status wdk.UTXOStatus, maxSatoshis uint64, limit int, excluded []uint) []*models.UserUTXO {
	t.Helper()
	result, err := fx.repos.FindLargestInsufficientUTXOsForUpdate(t.Context(), fx.db, fx.userID, wdk.BasketNameForChange, status, maxSatoshis, limit, excluded)
	require.NoError(t, err)
	return result
}

func outputIDs(utxos []*models.UserUTXO) []uint {
	ids := make([]uint, 0, len(utxos))
	for _, u := range utxos {
		ids = append(ids, u.OutputID)
	}
	return ids
}

func TestFindSmallestSufficientUTXOForUpdate(t *testing.T) {
	fx, cleanup := newBoundedUTXOsFixture(t)
	defer cleanup()

	t.Run("returns exact match when present", func(t *testing.T) {
		got := fx.sufficient(t, wdk.UTXOStatusMined, 1000, nil)
		require.NotNil(t, got)
		require.Equal(t, uint64(1000), got.Satoshis)
		require.Equal(t, fx.d, got.OutputID, "ties on satoshis must resolve to the lowest output_id")
	})

	t.Run("returns smallest sufficient when no exact match", func(t *testing.T) {
		got := fx.sufficient(t, wdk.UTXOStatusMined, 1050, nil)
		require.NotNil(t, got)
		require.Equal(t, fx.f, got.OutputID, "expected the 1200-sat row, the smallest >= 1050")
	})

	t.Run("respects exclusion list", func(t *testing.T) {
		got := fx.sufficient(t, wdk.UTXOStatusMined, 1000, []uint{fx.d})
		require.NotNil(t, got)
		require.Equal(t, fx.e, got.OutputID, "with D excluded the other 1000-sat row must be returned")

		got = fx.sufficient(t, wdk.UTXOStatusMined, 1050, []uint{fx.f})
		require.NotNil(t, got)
		require.Equal(t, fx.g, got.OutputID, "with F excluded the next sufficient row is the 5000-sat one")
	})

	t.Run("returns nil, nil when nothing is sufficient", func(t *testing.T) {
		got := fx.sufficient(t, wdk.UTXOStatusMined, 6000, nil)
		require.Nil(t, got)
	})

	t.Run("status tier isolation - mined query never returns unproven rows", func(t *testing.T) {
		// Only unproven J (4000) satisfies >= 2000 besides mined G (5000);
		// the mined query must pick G, and the unproven query must pick J.
		got := fx.sufficient(t, wdk.UTXOStatusMined, 2000, nil)
		require.NotNil(t, got)
		require.Equal(t, fx.g, got.OutputID)
		require.Equal(t, wdk.UTXOStatusMined, got.UTXOStatus)

		got = fx.sufficient(t, wdk.UTXOStatusUnproven, 2000, nil)
		require.NotNil(t, got)
		require.Equal(t, fx.j, got.OutputID)

		// No mined row >= 6000 exists even though it would be tempting to fall
		// back to another tier; the query must not.
		require.Nil(t, fx.sufficient(t, wdk.UTXOStatusMined, 5001, nil))
	})

	t.Run("skips reserved rows", func(t *testing.T) {
		// L (999, reserved) is the smallest row >= 999; it must be skipped in
		// favor of D (1000, unreserved).
		got := fx.sufficient(t, wdk.UTXOStatusMined, 999, nil)
		require.NotNil(t, got)
		require.Equal(t, fx.d, got.OutputID, "reserved 999-sat row must never be selected")
	})
}

func TestFindLargestInsufficientUTXOsForUpdate(t *testing.T) {
	fx, cleanup := newBoundedUTXOsFixture(t)
	defer cleanup()

	t.Run("returns rows below bound in descending order", func(t *testing.T) {
		got := fx.insufficient(t, wdk.UTXOStatusMined, 1000, 10, nil)
		require.Equal(t, []uint{fx.c, fx.b, fx.a}, outputIDs(got), "expected 300, 200, 100 in DESC order (reserved 999 skipped)")
	})

	t.Run("respects limit", func(t *testing.T) {
		got := fx.insufficient(t, wdk.UTXOStatusMined, 1000, 2, nil)
		require.Equal(t, []uint{fx.c, fx.b}, outputIDs(got))
	})

	t.Run("respects exclusion list", func(t *testing.T) {
		got := fx.insufficient(t, wdk.UTXOStatusMined, 1000, 10, []uint{fx.c})
		require.Equal(t, []uint{fx.b, fx.a}, outputIDs(got))
	})

	t.Run("breaks satoshi ties by output_id descending", func(t *testing.T) {
		got := fx.insufficient(t, wdk.UTXOStatusMined, 1050, 10, nil)
		require.Equal(t, []uint{fx.e, fx.d, fx.c, fx.b, fx.a}, outputIDs(got),
			"equal 1000-sat rows must be ordered by output_id DESC; reserved 999 row must be absent")
		require.NotContains(t, outputIDs(got), fx.l, "reserved row must never be returned")
	})

	t.Run("returns empty slice, nil error when nothing is below the bound", func(t *testing.T) {
		got := fx.insufficient(t, wdk.UTXOStatusMined, 100, 10, nil)
		require.Empty(t, got)
	})

	t.Run("status tier isolation - mined query never returns unproven or sending rows", func(t *testing.T) {
		got := fx.insufficient(t, wdk.UTXOStatusUnproven, 1000, 10, nil)
		require.Equal(t, []uint{fx.h}, outputIDs(got), "only unproven 150 is below 1000; unproven 1000 is not < 1000")

		got = fx.insufficient(t, wdk.UTXOStatusSending, 1000, 10, nil)
		require.Equal(t, []uint{fx.k}, outputIDs(got))

		// The mined query below 1000 must not contain H (150, unproven) or K (250, sending).
		got = fx.insufficient(t, wdk.UTXOStatusMined, 1000, 10, nil)
		require.NotContains(t, outputIDs(got), fx.h)
		require.NotContains(t, outputIDs(got), fx.k)
	})
}
