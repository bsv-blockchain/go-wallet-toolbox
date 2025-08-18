package wdk

import (
	"context"
	"fmt"
	"math"
	"time"

	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
)

const (
	nLockTimeThreshold uint32 = 500_000_000
)

// NLockTimeIsFinal checks if the provided value is a valid nLockTime and whether it is final.
func NLockTimeIsFinal(ctx context.Context, h HeightProvider, v any) (bool, error) {
	lockTime, final, err := parseLockTimeOrFinal(v)
	if err != nil {
		return false, fmt.Errorf("nlocktime: parse locktime: %w", err)
	}
	if final || lockTime == 0 {
		return true, nil
	}

	if lockTime >= nLockTimeThreshold {
		return nLockTimeIsFinalWithTimestamp(lockTime)
	}

	return nLockTimeIsFinalWithHeight(ctx, h, lockTime)
}

func nLockTimeIsFinalWithHeight(ctx context.Context, h HeightProvider, lockTime uint32) (bool, error) {
	height, err := h.CurrentHeight(ctx)
	if err != nil {
		return false, fmt.Errorf("nlocktime: current height: %w", err)
	}
	return lockTime <= height, nil
}

func nLockTimeIsFinalWithTimestamp(lockTime uint32) (bool, error) {
	now, err := to.UInt32(time.Now().Unix())
	if err != nil {
		return false, fmt.Errorf("nlocktime: convert current time: %w", err)
	}
	return lockTime <= now, nil
}

func parseLockTimeOrFinal(v any) (uint32, bool, error) {
	switch t := v.(type) {
	case uint32:
		return t, false, nil

	case int:
		if t < 0 {
			return 0, false, fmt.Errorf("nlocktime: negative locktime: %d", t)
		}
		u32, err := to.UInt32(t)
		if err != nil {
			return 0, false, fmt.Errorf("nlocktime: int to uint32: %w", err)
		}
		return u32, false, nil

	case *sdk.Transaction:
		return txLockTimeOrFinal(t)

	case string:
		tx, err := sdk.NewTransactionFromHex(t)
		if err != nil {
			return 0, false, fmt.Errorf("nlocktime: decode hex: %w", err)
		}
		return txLockTimeOrFinal(tx)

	case []byte:
		tx, err := sdk.NewTransactionFromBytes(t)
		if err != nil {
			return 0, false, fmt.Errorf("nlocktime: decode bytes: %w", err)
		}
		return txLockTimeOrFinal(tx)

	case []uint32:
		b, err := uint32SliceToBytes(t)
		if err != nil {
			return 0, false, err
		}
		tx, err := sdk.NewTransactionFromBytes(b)
		if err != nil {
			return 0, false, fmt.Errorf("nlocktime: decode []uint32: %w", err)
		}
		return txLockTimeOrFinal(tx)

	case []int:
		b, err := intSliceToBytes(t)
		if err != nil {
			return 0, false, err
		}
		tx, err := sdk.NewTransactionFromBytes(b)
		if err != nil {
			return 0, false, fmt.Errorf("nlocktime: decode []int: %w", err)
		}
		return txLockTimeOrFinal(tx)
	}
	return 0, false, fmt.Errorf("nlocktime: unsupported argument type: %T", v)
}

func txLockTimeOrFinal(tx *sdk.Transaction) (uint32, bool, error) {
	if inputsAllFinalAllowEmpty(tx) {
		return 0, true, nil
	}
	return tx.LockTime, false, nil
}

func inputsAllFinalAllowEmpty(tx *sdk.Transaction) bool {
	for _, in := range tx.Inputs {
		if in.SequenceNumber != math.MaxUint32 {
			return false
		}
	}
	return true
}

func uint32SliceToBytes(src []uint32) ([]byte, error) {
	out := make([]byte, len(src))
	for i, v := range src {
		if v > math.MaxUint8 {
			return nil, fmt.Errorf("nlocktime: invalid byte at %d: %d", i, v)
		}
		out[i] = byte(v)
	}
	return out, nil
}

func intSliceToBytes(src []int) ([]byte, error) {
	out := make([]byte, len(src))
	for i, v := range src {
		if v < 0 || v > math.MaxUint8 {
			return nil, fmt.Errorf("nlocktime: invalid byte at %d: %d", i, v)
		}
		out[i] = byte(v)
	}
	return out, nil
}
