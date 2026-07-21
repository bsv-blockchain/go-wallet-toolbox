package main

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/stretchr/testify/require"
)

func TestOpReturnLockingScriptContainsPayload(t *testing.T) {
	locking, err := OpReturnLockingScript(ProofPayload)
	require.NoError(t, err)
	require.NotEmpty(t, locking)

	s := script.NewFromBytes(locking)
	// OP_FALSE OP_RETURN <payload>
	// go-sdk v1.3 ToASM returns string (no error)
	asm := s.ToASM()
	require.Contains(t, asm, "OP_RETURN")
	// payload appears as hex in ASM for pushdata — also check raw contains bytes
	require.Contains(t, string(locking), ProofPayload)
	require.Equal(t, "the proof is in the pudding", ProofPayload)
}

func TestOpReturnLockingScriptRejectsEmpty(t *testing.T) {
	_, err := OpReturnLockingScript("")
	require.Error(t, err)
}
