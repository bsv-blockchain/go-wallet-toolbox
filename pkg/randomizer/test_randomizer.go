package randomizer

import (
	"encoding/base64"
	"fmt"
	"sync"
)

const (
	minRandomizeLength = 3 // minimum length for randomization to avoid early overflow
)

// TestRandomizer is a test implementation of the Randomizer interface.
// It provides deterministic outputs for testing purposes.
type TestRandomizer struct {
	base64Locker  sync.Mutex
	baseCharacter byte
	rollCounter   int
}

// NewTestRandomizer creates and returns a new instance of TestRandomizer.
func NewTestRandomizer() *TestRandomizer {
	return &TestRandomizer{
		baseCharacter: 'a',
	}
}

// Bytes returns a deterministic slice of bytes of the specified length for testing purposes.
// Returns an error if length is zero.
func (t *TestRandomizer) Bytes(length uint64) ([]byte, error) {
	if length == 0 {
		return nil, fmt.Errorf("length cannot be zero")
	}

	return t.nextBytes(length), nil
}

// Base64 generates a deterministic base64-encoded string of the specified length.
// The content of the string is a repeated sequence of the character 'a'.
func (t *TestRandomizer) Base64(length uint64) (string, error) {
	randomBytes, err := t.Bytes(length)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(randomBytes), nil
}

func (t *TestRandomizer) nextBytes(length uint64) []byte {
	if length == 0 {
		panic("length cannot be zero for random bytes generation")
	}

	t.base64Locker.Lock()
	defer t.base64Locker.Unlock()

	current := t.baseCharacter
	currentRollCounter := t.rollCounter

	if t.baseCharacter < 0x7F {
		t.baseCharacter++
	} else {
		t.baseCharacter = 0x21

		if length < minRandomizeLength {
			panic("test randomizes base character overflow - too short length for randomization")
		}
		if t.rollCounter == 0xFF {
			panic("test randomizes base character overflow - too many calls for randomization")
		}
		t.rollCounter++
	}

	result := make([]byte, length)
	for i := range result {
		result[i] = current
		if currentRollCounter > 0 {
			result[0] = 0x20
			result[1] = byte(currentRollCounter % 0xFF)
		}
	}

	return result
}

// Shuffle performs a deterministic shuffle operation on a slice of size n.
// It calls the provided swap function twice for each pair of indices to preserve the original order.
func (t *TestRandomizer) Shuffle(n int, swap func(i int, j int)) {
	for i := 0; i < n-1; i++ {
		swap(i, i+1)
		swap(i, i+1)
	}
}

// Uint64 returns a deterministic uint64 value, which is always 0 in this implementation.
func (t *TestRandomizer) Uint64(max uint64) uint64 {
	return 0
}
