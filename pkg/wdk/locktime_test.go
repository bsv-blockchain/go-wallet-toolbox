package wdk_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestNLockTimeIsFinal_ZeroAndFinal(t *testing.T) {
	// given:
	ctx := t.Context()
	h := testutils.NewStubHeight(700_000, nil)
	txFinal := testutils.NewTestTransactionWithLocktime(t, 999_999, math.MaxUint32, math.MaxUint32)

	// when:
	gotZero, errZero := wdk.NLockTimeIsFinal(ctx, h, uint32(0))
	gotTx, errTx := wdk.NLockTimeIsFinal(ctx, h, txFinal)

	// then:
	require.NoError(t, errZero)
	require.True(t, gotZero)
	require.NoError(t, errTx)
	require.True(t, gotTx)
}

func TestNLockTimeIsFinal_HeightComparisons(t *testing.T) {
	// given:
	ctx := t.Context()
	const chainHeight = uint32(700_000)
	h := testutils.NewStubHeight(chainHeight, nil)

	cases := []struct {
		name     string
		locktime uint32
		want     bool
	}{
		{"nLockTime < height -> final", chainHeight - 1, true},
		{"nLockTime == height -> final", chainHeight, true},
		{"nLockTime > height -> NOT final", chainHeight + 1, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when:
			got, err := wdk.NLockTimeIsFinal(ctx, h, tc.locktime)

			// then:
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestNLockTimeIsFinal_TimestampComparisons(t *testing.T) {
	// given:
	ctx := t.Context()
	h := testutils.NewStubHeight(0, nil)
	now := uint32(time.Now().Unix()) //nolint:gosec // unix timestamp fits in uint32 until year 2106
	earlier := now - 60
	later := now + 60

	// when:
	gotEarlier, errEarlier := wdk.NLockTimeIsFinal(ctx, h, earlier)
	now2 := uint32(time.Now().Unix()) //nolint:gosec // unix timestamp fits in uint32 until year 2106
	gotEqual, errEqual := wdk.NLockTimeIsFinal(ctx, h, now2)
	gotLater, errLater := wdk.NLockTimeIsFinal(ctx, h, later)

	// then:
	require.NoError(t, errEarlier)
	require.True(t, gotEarlier)
	require.NoError(t, errEqual)
	require.True(t, gotEqual)
	require.NoError(t, errLater)
	require.False(t, gotLater)
}

func TestNLockTimeIsFinal_Int_PositiveAndNegative(t *testing.T) {
	// given:
	ctx := t.Context()
	h := testutils.NewStubHeight(600_000, nil)

	// when:
	gotOK, errOK := wdk.NLockTimeIsFinal(ctx, h, int(600_000))
	_, errNeg := wdk.NLockTimeIsFinal(ctx, h, int(-1))

	// then:
	require.NoError(t, errOK)
	require.True(t, gotOK)
	require.Error(t, errNeg)
}

func TestNLockTimeIsFinal_TxPointer_NonFinalInput(t *testing.T) {
	// given:
	ctx := t.Context()
	h := testutils.NewStubHeight(700_000, nil)

	tx := testutils.NewTestTransactionWithLocktime(t, 700_001, math.MaxUint32, math.MaxUint32-1)

	// when:
	got, err := wdk.NLockTimeIsFinal(ctx, h, tx)

	// then:
	require.NoError(t, err)
	require.False(t, got)
}

func TestNLockTimeIsFinal_String_ErrorPaths(t *testing.T) {
	// given:
	ctx := t.Context()
	h := testutils.NewStubHeight(0, nil)

	// when:
	_, errInvalidHex := wdk.NLockTimeIsFinal(ctx, h, "zz")
	_, errShortHex := wdk.NLockTimeIsFinal(ctx, h, "00")

	// then:
	require.Error(t, errInvalidHex)
	require.Error(t, errShortHex)
}

func TestNLockTimeIsFinal_Bytes_ErrorPaths(t *testing.T) {
	// given:
	ctx := t.Context()
	h := testutils.NewStubHeight(0, nil)

	// when:
	_, errEmpty := wdk.NLockTimeIsFinal(ctx, h, []byte{})
	_, errGarbage := wdk.NLockTimeIsFinal(ctx, h, []byte{0x00})

	// then:
	require.Error(t, errEmpty)
	require.Error(t, errGarbage)
}

func TestNLockTimeIsFinal_Uint32Slice_ErrorPaths(t *testing.T) {
	// given:
	ctx := t.Context()
	h := testutils.NewStubHeight(0, nil)

	// when:
	_, errRange := wdk.NLockTimeIsFinal(ctx, h, []uint32{256})
	_, errDecode := wdk.NLockTimeIsFinal(ctx, h, []uint32{0x00, 0x01})

	// then:
	require.Error(t, errRange)
	require.Error(t, errDecode)
}

func TestNLockTimeIsFinal_IntSlice_ErrorPaths(t *testing.T) {
	// given:
	ctx := t.Context()
	h := testutils.NewStubHeight(0, nil)

	// when:
	_, errNeg := wdk.NLockTimeIsFinal(ctx, h, []int{-1})
	_, errDecode := wdk.NLockTimeIsFinal(ctx, h, []int{0x00, 0x01})

	// then:
	require.Error(t, errNeg)
	require.Error(t, errDecode)
}
