package wdk

// Randomizer contains functions for randomization
type Randomizer interface {
	Base64(length uint64) (string, error)
	Shuffle(n int, swap func(i, j int))
	Uint64(max uint64) uint64
	Bytes(length uint64) ([]byte, error)
}
