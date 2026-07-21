package stream_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/stream"
	"github.com/stretchr/testify/require"
)

func TestHashPayloadIsSHA256OfIterationConcatTimestamp(t *testing.T) {
	ts := time.Date(2026, 7, 21, 12, 0, 0, 123456789, time.UTC)
	got := stream.HashPayload(42, ts)

	msg := "42" + ts.Format(time.RFC3339Nano)
	want := sha256.Sum256([]byte(msg))
	require.Equal(t, want[:], got)
	require.Len(t, got, 32)
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
}
