package randomizer_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
)

func TestRandomBase64(t *testing.T) {
	// given:
	random := randomizer.New()

	// when:
	randomized, err := random.Base64(16)

	// then:
	require.NoError(t, err)
	require.NotEmpty(t, randomized)
}

func TestRandomBase64Uniqueness(t *testing.T) {
	// given:
	random := randomizer.New()

	// when:
	randomized1, err := random.Base64(16)
	require.NoError(t, err)

	randomized2, err := random.Base64(16)
	require.NoError(t, err)

	// then:
	require.NotEqual(t, randomized1, randomized2)
}

func TestRandomBase64OnZeroLength(t *testing.T) {
	// given:
	random := randomizer.New()

	// when:
	_, err := random.Base64(0)

	// then:
	require.Error(t, err)
}

func TestRandomBase64Lengths(t *testing.T) {
	// given:
	random := randomizer.New()

	for length := uint64(1); length <= 100; length++ {
		// when:
		randomized, err := random.Base64(length)

		// then:
		require.NoError(t, err)

		// NOTE: Base64 encoding adds padding, so the length sequence is as follows:
		// Length -> Base64 Length
		// 1 -> 4
		// 2 -> 4
		// 3 -> 4
		// 4 -> 8
		// 5 -> 8
		// 6 -> 8
		// 7 -> 12
		// ...
		expectedBase64Length := ((length-1)/3 + 1) * 4
		require.Equal(t, expectedBase64Length, uint64(len(randomized)))
	}
}

func TestShuffle(t *testing.T) {
	// given:
	random := randomizer.New()

	// and:
	numbers := make([]int, 100)
	for i := 0; i < 100; i++ {
		numbers[i] = i
	}

	// when:
	random.Shuffle(len(numbers), func(i, j int) {
		numbers[i], numbers[j] = numbers[j], numbers[i]
	})

	// then:
	for i := 0; i < len(numbers); i++ {
		if numbers[i] != i {
			t.Log("Shuffled - it's ok")
			return
		}
	}
	require.Fail(t, "Numbers are not shuffled")
}

func TestRandomUint64(t *testing.T) {
	// given:
	random := randomizer.New()

	// and:
	maxValue := uint64(100)

	// when:
	uniqueValues := make(map[uint64]struct{})
	for i := 0; i < 100; i++ {
		value := random.Uint64(maxValue)
		require.LessOrEqual(t, value, maxValue)

		uniqueValues[value] = struct{}{}
	}

	// then:
	require.Greater(t, len(uniqueValues), 1, "Random values should not be constant")
}
