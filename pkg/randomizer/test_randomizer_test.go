package randomizer_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
)

func TestRandomBase64ByTestRandomizer(t *testing.T) {
	// given:
	random := randomizer.NewTestRandomizer()

	// when:
	randomized, err := random.Base64(16)

	// then:
	require.NoError(t, err)
	require.Equal(t, "YWFhYWFhYWFhYWFhYWFhYQ==", randomized)
}

func TestRandomBase64OnZeroLengthByTestRandomizer(t *testing.T) {
	// given:
	random := randomizer.NewTestRandomizer()

	// when:
	_, err := random.Base64(0)

	// then:
	require.Error(t, err)
}

func TestLengthOfBase64TestImplEqualsDefaultRandomizer(t *testing.T) {
	// given:
	random := randomizer.New()
	testRandom := randomizer.NewTestRandomizer()

	for length := uint64(1); length <= 100; length++ {
		// when:
		actual, err := random.Base64(length)

		// then:
		require.NoError(t, err)

		// when:
		test, err := testRandom.Base64(length)

		// then:
		require.NoError(t, err)

		// and:
		require.Len(t, actual, len(test))
	}
}

func TestShuffleByTestRandomizer(t *testing.T) {
	// given:
	random := randomizer.NewTestRandomizer()

	// and:
	original := make([]int, 100)

	for i := 0; i < 100; i++ {
		original[i] = i
	}

	// and:
	numbers := make([]int, 100)
	copy(numbers, original)

	// when:
	swapFcnCalled := false
	random.Shuffle(len(numbers), func(i, j int) {
		swapFcnCalled = true
		numbers[i], numbers[j] = numbers[j], numbers[i]
	})

	// then:
	require.True(t, swapFcnCalled)
	require.Equal(t, original, numbers, "Numbers should be in the same order")
}

func TestRandomUint64ByTestRandomizer(t *testing.T) {
	// given:
	random := randomizer.NewTestRandomizer()

	// when:
	value := random.Uint64(1000)

	// then:
	require.Equal(t, uint64(0), value, "Random value should be 0")
}

func TestRandomManyBase64Values(t *testing.T) {
	// given:
	random := randomizer.NewTestRandomizer()

	lookup := make(map[string]struct{})
	const (
		baseCharacterRange       = 0x7F - 0x21
		combinationOfRollCounter = baseCharacterRange + 1
		count                    = combinationOfRollCounter * 255 // this is the maximum number of unique combinations for test randomizer
		length                   = 3
	)

	// when:
	for i := 0; i < count; i++ {
		randomized, err := random.Base64(length)
		require.NoError(t, err)

		// then:
		require.NotEmpty(t, randomized, "Randomized value should not be empty")

		// and:
		_, exists := lookup[randomized]
		require.False(t, exists, "Randomized value should be unique")

		// and:
		lookup[randomized] = struct{}{}
	}
}
