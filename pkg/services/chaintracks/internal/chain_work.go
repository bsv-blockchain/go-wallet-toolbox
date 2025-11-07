package internal

import (
	"fmt"
	"math/big"
)

type ChainWork struct {
	big.Int
}

func ChainWorkFromHex(chainworkHex string) ChainWork {
	var bigInt big.Int
	bigInt.SetString(chainworkHex, 16)
	return ChainWork{bigInt}
}

func ChainWorkFromBits(bits uint32) ChainWork {
	return convertBitsToWork(bits)
}

func (cw ChainWork) AddChainWork(other ChainWork) ChainWork {
	var result big.Int
	result.Add(&cw.Int, &other.Int)
	return ChainWork{result}
}

func (cw ChainWork) CmpChainWork(other ChainWork) int {
	return cw.Cmp(&other.Int)
}

func (cw ChainWork) To64PadHex() string {
	return formatChainWorkHex(cw.Int)
}

func convertBitsToWork(bits uint32) ChainWork {
	// 1. Calculate Target from bits
	// Extract exponent and mantissa
	shift := bits >> 24 & 0xff
	data := bits & 0x007fffff

	// Calculate target: T = mantissa * 2^(8 * (exponent - 3))
	target := big.NewInt(int64(data))
	if shift <= 3 {
		exp := 3 - uint(shift)
		target.Rsh(target, exp*8)
	} else {
		exp := uint(shift) - 3
		target.Lsh(target, exp*8)
	}

	// Create 2^256 (which is 1 << 256)
	two256 := new(big.Int).Lsh(big.NewInt(1), 256)

	// Create target + 1
	targetPlusOne := new(big.Int).Add(target, big.NewInt(1))

	// work = 2^256 / (target + 1) (integer division)
	work := new(big.Int).Div(two256, targetPlusOne)

	// 3. Return as ChainWork
	return ChainWork{*work}
}

func formatChainWorkHex(value big.Int) string {
	return fmt.Sprintf("%064x", &value)
}
