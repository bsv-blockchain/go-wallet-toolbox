package primitives

import "fmt"

// PositiveIntegerDefault10Max10000  is a positive integer that defaults to 10, and has an upper bound of 10000.
type PositiveIntegerDefault10Max10000 uint

// Validate checks if the integer is maximum 10000
func (i PositiveIntegerDefault10Max10000) Validate() error {
	if i > 10000 {
		return fmt.Errorf("is larger than 10000")
	}

	return nil
}

// PositiveInteger represents a positive integer value
type PositiveInteger uint64

// SatoshiValue Represents a value in Satoshis, constrained by the max supply of Bitcoin (2.1 * 10^15 Satoshis).
// @maximum 2100000000000000
type SatoshiValue uint64

// MaxSatoshis is the maximum number of Satoshis in the Bitcoin supply
const MaxSatoshis = 2100000000000000

// SweepMaxSatoshis is a special value that when used as an output's value will sweep all available wallet funds to that output.
const SweepMaxSatoshis = 2099999999999999

// Validate checks if the value is less than the maximum number of Satoshis
func (s SatoshiValue) Validate() error {
	if s > MaxSatoshis {
		return fmt.Errorf("less than %d", MaxSatoshis)
	}
	return nil
}
