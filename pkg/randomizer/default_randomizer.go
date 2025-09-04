// Package randomizer provides utilities for generating secure random values
// and performing randomization operations safely.
package randomizer

import (
	cryptorand "crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"math/rand"

	"github.com/go-softwarelab/common/pkg/must"
)

// DefaultRandomizer implements cryptographically secure random operations.
type DefaultRandomizer struct{}

// New creates and returns a new DefaultRandomizer instance.
func New() *DefaultRandomizer {
	return &DefaultRandomizer{}
}

// Bytes generates a slice of cryptographically secure random bytes of the specified length.
// Returns an error if length is zero or if the random byte generation fails.
func (s *DefaultRandomizer) Bytes(length uint64) ([]byte, error) {
	if length == 0 {
		return nil, fmt.Errorf("length cannot be zero")
	}

	randomBytes := make([]byte, length)
	_, err := cryptorand.Read(randomBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read random bytes: %w", err)
	}

	return randomBytes, nil
}

// Base64 generates a random byte sequence of specified length and returns it as a base64 encoded string.
// Returns an error if random bytes cannot be generated or if length is zero.
func (s *DefaultRandomizer) Base64(length uint64) (string, error) {
	randomBytes, err := s.Bytes(length)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(randomBytes), nil
}

// Shuffle randomizes the order of n elements using the provided swap function.
// This is a wrapper around the standard library's rand.Shuffle.
func (s *DefaultRandomizer) Shuffle(n int, swap func(i int, j int)) {
	rand.Shuffle(n, swap)
}

// Uint64 generates a cryptographically secure random unsigned integer between 0 and max-1.
// Panics if random number generation fails.
func (s *DefaultRandomizer) Uint64(max uint64) uint64 {
	nBig, err := cryptorand.Int(cryptorand.Reader, big.NewInt(must.ConvertToInt64FromUnsigned(max)))
	if err != nil {
		panic(fmt.Errorf("failed to generate random number: %w", err))
	}
	return nBig.Uint64()
}
