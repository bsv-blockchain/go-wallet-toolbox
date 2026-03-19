package satoshi_test

import (
	"math"
	"testing"

	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestAdd(t *testing.T) {
	t.Run("add two ints", func(t *testing.T) {
		c, err := satoshi.Add(1, 2)
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(3), c)
	})

	t.Run("add int and uint64", func(t *testing.T) {
		c, err := satoshi.Add(1, uint64(2))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(3), c)
	})

	t.Run("add uint and negative int", func(t *testing.T) {
		c, err := satoshi.Add(uint(1), -2)
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(-1), c)
	})

	t.Run("add two uints", func(t *testing.T) {
		c, err := satoshi.Add(uint(1), uint(2))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(3), c)
	})

	t.Run("add two max satoshis", func(t *testing.T) {
		_, err := satoshi.Add(primitives.MaxSatoshis, primitives.MaxSatoshis)
		require.Error(t, err)
	})

	t.Run("add two minimum satoshi values", func(t *testing.T) {
		_, err := satoshi.Add(-primitives.MaxSatoshis, -primitives.MaxSatoshis)
		require.Error(t, err)
	})

	t.Run("add two negative ints", func(t *testing.T) {
		c, err := satoshi.Add(-1, -2)
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(-3), c)
	})

	t.Run("add zero and int", func(t *testing.T) {
		c, err := satoshi.Add(0, 5)
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(5), c)
	})

	t.Run("add max satoshis and minus-max-satoshis", func(t *testing.T) {
		c, err := satoshi.Add(primitives.MaxSatoshis, -primitives.MaxSatoshis)
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(0), c)
	})
}

func TestSubtract(t *testing.T) {
	t.Run("subtract two ints", func(t *testing.T) {
		c, err := satoshi.Subtract(5, 3)
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(2), c)
	})

	t.Run("subtract int and uint64", func(t *testing.T) {
		c, err := satoshi.Subtract(5, uint64(2))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(3), c)
	})

	t.Run("subtract uint and negative int", func(t *testing.T) {
		// 1 - (-2) equals 3
		c, err := satoshi.Subtract(uint(1), -2)
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(3), c)
	})

	t.Run("subtract resulting in zero", func(t *testing.T) {
		c, err := satoshi.Subtract(2, 2)
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(0), c)
	})

	t.Run("subtract to obtain a negative result", func(t *testing.T) {
		c, err := satoshi.Subtract(3, 5)
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(-2), c)
	})

	t.Run("subtract exceeding max positive value", func(t *testing.T) {
		// primitives.MaxSatoshis - (-1) equals primitives.MaxSatoshis + 1 (overflow)
		_, err := satoshi.Subtract(primitives.MaxSatoshis, -1)
		require.Error(t, err)
	})

	t.Run("subtract exceeding max negative value", func(t *testing.T) {
		// (-primitives.MaxSatoshis) - 1 equals -(primitives.MaxSatoshis + 1) (underflow)
		_, err := satoshi.Subtract(-primitives.MaxSatoshis, 1)
		require.Error(t, err)
	})
}

type otherTypeAlias int64

func TestFrom(t *testing.T) {
	t.Run("from int", func(t *testing.T) {
		c, err := satoshi.From(int64(1))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(1), c)
	})

	t.Run("from uint", func(t *testing.T) {
		c, err := satoshi.From(uint64(1))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(1), c)
	})

	t.Run("from negative int", func(t *testing.T) {
		c, err := satoshi.From(int64(-1))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(-1), c)
	})

	t.Run("from max uint32", func(t *testing.T) {
		c, err := satoshi.From(uint32(math.MaxUint32))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(4294967295), c)
	})

	t.Run("from max int32", func(t *testing.T) {
		c, err := satoshi.From(int32(math.MaxInt32))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(2147483647), c)
	})

	t.Run("from min int32", func(t *testing.T) {
		c, err := satoshi.From(int32(math.MinInt32))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(-2147483648), c)
	})

	t.Run("from max int64", func(t *testing.T) {
		_, err := satoshi.From(int64(math.MaxInt64))
		require.Error(t, err)
	})

	t.Run("from min int64", func(t *testing.T) {
		_, err := satoshi.From(int64(math.MinInt64))
		require.Error(t, err)
	})

	t.Run("from max uint64", func(t *testing.T) {
		_, err := satoshi.From(uint64(math.MaxUint64))
		require.Error(t, err)
	})

	t.Run("from max satoshi", func(t *testing.T) {
		c, err := satoshi.From(primitives.MaxSatoshis)
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(primitives.MaxSatoshis), c)
	})

	t.Run("from negative max satoshi", func(t *testing.T) {
		c, err := satoshi.From(-primitives.MaxSatoshis)
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(-primitives.MaxSatoshis), c)
	})

	t.Run("from max satoshi + 1", func(t *testing.T) {
		_, err := satoshi.From(primitives.MaxSatoshis + 1)
		require.Error(t, err)
	})

	t.Run("from negative max satoshi - 1", func(t *testing.T) {
		_, err := satoshi.From(-primitives.MaxSatoshis - 1)
		require.Error(t, err)
	})

	t.Run("from max satoshi as uint64", func(t *testing.T) {
		c, err := satoshi.From(uint64(primitives.MaxSatoshis))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(primitives.MaxSatoshis), c)
	})

	t.Run("from max satoshi as int64", func(t *testing.T) {
		c, err := satoshi.From(int64(primitives.MaxSatoshis))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(primitives.MaxSatoshis), c)
	})

	t.Run("from negative max satoshi as int64", func(t *testing.T) {
		c, err := satoshi.From(int64(-primitives.MaxSatoshis))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(-primitives.MaxSatoshis), c)
	})

	t.Run("from other type alias", func(t *testing.T) {
		c, err := satoshi.From(otherTypeAlias(1))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(1), c)
	})

	t.Run("from other type alias equals max satoshi + 1", func(t *testing.T) {
		_, err := satoshi.From(otherTypeAlias(primitives.MaxSatoshis + 1))
		require.Error(t, err)
	})
}

func TestSum(t *testing.T) {
	t.Run("sum of empty sequence", func(t *testing.T) {
		c, err := satoshi.Sum(seq.Of[int]())
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(0), c)
	})

	t.Run("sum of single element sequence", func(t *testing.T) {
		c, err := satoshi.Sum(seq.Of(1))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(1), c)
	})

	t.Run("sum of multiple elements", func(t *testing.T) {
		c, err := satoshi.Sum(seq.Of(1, 2, 3))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(6), c)
	})

	t.Run("sum of multiple elements with different signs", func(t *testing.T) {
		c, err := satoshi.Sum(seq.Of(1, -2, 3))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(2), c)
	})

	t.Run("sum of multiple max satoshis", func(t *testing.T) {
		c, err := satoshi.Sum(seq.Of(primitives.MaxSatoshis, -primitives.MaxSatoshis, primitives.MaxSatoshis, -primitives.MaxSatoshis))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(0), c)
	})

	t.Run("sum exceeding max satoshi value", func(t *testing.T) {
		_, err := satoshi.Sum(seq.Of(primitives.MaxSatoshis, 1))
		require.Error(t, err)
	})

	t.Run("sum of only negative values", func(t *testing.T) {
		c, err := satoshi.Sum(seq.Of(-1, -2, -3))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(-6), c)
	})

	t.Run("sum of single max satoshi value", func(t *testing.T) {
		c, err := satoshi.Sum(seq.Of(primitives.MaxSatoshis))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(primitives.MaxSatoshis), c)
	})
}

func TestMultiply(t *testing.T) {
	t.Run("multiply two ints", func(t *testing.T) {
		res, err := satoshi.Multiply(2, 3)
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(6), res)
	})

	t.Run("multiplication overflow", func(t *testing.T) {
		_, err := satoshi.Multiply(primitives.MaxSatoshis, 2)
		require.Error(t, err)
	})

	t.Run("multiply two int64 values", func(t *testing.T) {
		res, err := satoshi.Multiply(int64(3), int64(4))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(12), res)
	})

	t.Run("multiply int and int64", func(t *testing.T) {
		res, err := satoshi.Multiply(3, int64(5))
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(15), res)
	})

	t.Run("multiply uint and int", func(t *testing.T) {
		res, err := satoshi.Multiply(uint(2), 4)
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(8), res)
	})

	t.Run("multiply uint64 and int", func(t *testing.T) {
		res, err := satoshi.Multiply(uint64(2), 4)
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(8), res)
	})

	t.Run("multiply negative and positive", func(t *testing.T) {
		res, err := satoshi.Multiply(-2, 4)
		require.NoError(t, err)
		require.Equal(t, satoshi.Value(-8), res)
	})
}

func TestMustMultiply(t *testing.T) {
	t.Run("must multiply happy case", func(t *testing.T) {
		res := satoshi.MustMultiply(4, 5)
		require.Equal(t, satoshi.Value(20), res)
	})

	t.Run("must multiply panics on overflow", func(t *testing.T) {
		require.Panics(t, func() {
			_ = satoshi.MustMultiply(primitives.MaxSatoshis, 2)
		})
	})
}

func TestEqual(t *testing.T) {
	t.Run("equal numbers", func(t *testing.T) {
		eq, err := satoshi.Equal(10, 10)
		require.NoError(t, err)
		require.True(t, eq)
	})

	t.Run("unequal numbers", func(t *testing.T) {
		eq, err := satoshi.Equal(10, 20)
		require.NoError(t, err)
		require.False(t, eq)
	})

	t.Run("equal with different types", func(t *testing.T) {
		eq, err := satoshi.Equal(10, int64(10))
		require.NoError(t, err)
		require.True(t, eq)
	})

	t.Run("equal with max satoshis", func(t *testing.T) {
		eq, err := satoshi.Equal(primitives.MaxSatoshis, primitives.MaxSatoshis)
		require.NoError(t, err)
		require.True(t, eq)
	})

	t.Run("equal with negative max satoshis", func(t *testing.T) {
		eq, err := satoshi.Equal(-primitives.MaxSatoshis, -primitives.MaxSatoshis)
		require.NoError(t, err)
		require.True(t, eq)
	})

	t.Run("try equal with max satoshis + 1", func(t *testing.T) {
		_, err := satoshi.Equal(0, primitives.MaxSatoshis+1)
		require.Error(t, err)
	})
}

// Added tests for MustEqual
func TestMustEqual(t *testing.T) {
	t.Run("must equal happy case", func(t *testing.T) {
		eq := satoshi.MustEqual(15, 15)
		require.True(t, eq)
	})

	t.Run("try must equal with max satoshis + 1", func(t *testing.T) {
		require.Panics(t, func() {
			_ = satoshi.MustEqual(0, primitives.MaxSatoshis+1)
		})
	})
}

func TestUInt64(t *testing.T) {
	t.Run("positive value", func(t *testing.T) {
		v := satoshi.Value(100)
		u, err := v.UInt64()
		require.NoError(t, err)
		require.Equal(t, uint64(100), u)
	})

	t.Run("negative value", func(t *testing.T) {
		v := satoshi.Value(-50)
		u, err := v.UInt64()
		require.Error(t, err)
		require.Equal(t, uint64(0), u)
	})
}

func TestMustUInt64(t *testing.T) {
	t.Run("must uint64 happy case", func(t *testing.T) {
		v := satoshi.Value(200)
		require.Equal(t, uint64(200), v.MustUInt64())
	})

	t.Run("must uint64 panics on negative", func(t *testing.T) {
		require.Panics(t, func() {
			_ = satoshi.Value(-1).MustUInt64()
		})
	})
}
