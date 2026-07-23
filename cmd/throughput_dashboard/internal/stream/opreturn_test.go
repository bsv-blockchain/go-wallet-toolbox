package stream_test

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/stream"
)

func TestHashPayloadIsSHA256OfIterationConcatTimestamp(t *testing.T) {
	ts := time.Date(2026, 7, 21, 12, 0, 0, 123456789, time.UTC)
	got := stream.HashPayload(42, ts)

	msg := strconv.FormatUint(42, 10) + ts.UTC().Format(time.RFC3339Nano)
	want := sha256.Sum256([]byte(msg))
	require.Equal(t, want[:], got)
	require.Len(t, got, sha256.Size)
}

func TestHashPayloadIsDeterministic(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC)
	a := stream.HashPayload(99, ts)
	b := stream.HashPayload(99, ts)
	require.Equal(t, a, b)
}

func TestHashPayloadUsesUTC(t *testing.T) {
	// Same instant in different zones must hash identically (UTC format).
	utc := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	offset := time.FixedZone("UTC-5", -5*60*60)
	local := utc.In(offset)
	require.Equal(t, stream.HashPayload(1, utc), stream.HashPayload(1, local))

	// Explicit expected message uses UTC layout.
	msg := "1" + utc.Format(time.RFC3339Nano)
	want := sha256.Sum256([]byte(msg))
	require.Equal(t, want[:], stream.HashPayload(1, local))
}

func TestHashPayloadChangesWithIterationAndTime(t *testing.T) {
	ts := time.Unix(1_700_000_000, 0).UTC()
	a := stream.HashPayload(1, ts)
	b := stream.HashPayload(2, ts)
	c := stream.HashPayload(1, ts.Add(time.Second))
	require.NotEqual(t, hex.EncodeToString(a), hex.EncodeToString(b))
	require.NotEqual(t, hex.EncodeToString(a), hex.EncodeToString(c))
}

func TestOpReturnLockingScriptForHashContainsHash(t *testing.T) {
	hash := stream.HashPayload(7, time.Unix(0, 0).UTC())
	locking, err := stream.OpReturnLockingScriptForHash(hash)
	require.NoError(t, err)
	require.NotEmpty(t, locking)

	asm := script.NewFromBytes(locking).ToASM()
	require.Contains(t, asm, "OP_RETURN")
	require.Contains(t, string(locking), string(hash))
}

func TestOpReturnLockingScriptForHashRejectsEmpty(t *testing.T) {
	_, err := stream.OpReturnLockingScriptForHash(nil)
	require.Error(t, err)

	_, err = stream.OpReturnLockingScriptForHash([]byte{})
	require.Error(t, err)
}

func TestOpReturnLockingScriptForHashRejectsWrongLength(t *testing.T) {
	_, err := stream.OpReturnLockingScriptForHash([]byte("too-short"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "32")

	_, err = stream.OpReturnLockingScriptForHash(make([]byte, 64))
	require.Error(t, err)
}

func TestOpReturnLockingScriptForIteration(t *testing.T) {
	ts := time.Date(2026, 7, 21, 15, 30, 0, 1, time.UTC)
	locking, err := stream.OpReturnLockingScriptForIteration(123, ts)
	require.NoError(t, err)

	hash := stream.HashPayload(123, ts)
	require.Contains(t, string(locking), string(hash))
	require.Contains(t, script.NewFromBytes(locking).ToASM(), "OP_RETURN")
}
