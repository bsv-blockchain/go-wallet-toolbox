package txutils

import (
	"fmt"
	"iter"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/go-softwarelab/common/pkg/seq"
)

type Randomizer func(max uint64) uint64

type ChangeDistribution struct {
	initialValue satoshi.Value
	randomizer   Randomizer
}

func NewChangeDistribution(initialValue satoshi.Value, randomizer Randomizer) *ChangeDistribution {
	return &ChangeDistribution{
		initialValue: initialValue,
		randomizer:   randomizer,
	}
}

func (d *ChangeDistribution) Distribute(count uint64, amount satoshi.Value) iter.Seq[satoshi.Value] {
	if count == 0 || amount == 0 {
		return seq.Of[satoshi.Value]()
	}
	if count == 1 {
		return seq.Of(amount)
	}

	// saturation: a moment when all the outputs are equal to initialValue
	saturationThreshold := satoshi.MustMultiply(count, d.initialValue)

	switch {
	case amount == saturationThreshold:
		return seq.Repeat(d.initialValue, count)
	case amount > saturationThreshold:
		return d.saturatedRandomDistribution(count, amount)
	default:
		return d.notSaturatedDistribution(count, amount)
	}
}

// saturatedRandomDistribution - generate randomized outputs with given constraints:
// 1. each output is >= initialValue
// 2. sum of all outputs = amount
// 3. number of outputs = count
func (d *ChangeDistribution) saturatedRandomDistribution(count uint64, amount satoshi.Value) iter.Seq[satoshi.Value] {
	amountUint64 := amount.MustUInt64()
	base := amountUint64 / count
	remainder := amountUint64 % count

	// initial distribution that will be modified by random noise
	// e.g. For 3 outputs and 20 amount, we have:
	// base = 6, remainder = 2, then:
	// distribution = [8, 6, 6]
	distribution := seq.Concat(
		seq.Of[uint64](base+remainder),
		seq.Repeat(base, count-1),
	)

	// randomize the noise for each output
	// e.g. for given distribution [8, 6, 6] and initialValue = 4:
	// noise will be randomized with following ranges:
	// [<0, 4>, <0, 2>, <0, 2>]
	noise := d.randomNoise(count, distribution)

	// add noise to the distribution
	// e.g. for given distribution [8, 6, 6] and noise [3, 1, 2]:
	// final distribution will be:
	// [8-3+2, 6-1+1, 6-2+3] = [7, 6, 7]
	var i uint64
	var v uint64
	return seq.Map(distribution, func(current uint64) satoshi.Value {
		// noise[i] - random value for current output (subtraction does not make it less than initialValue)
		// noise[reverseIndex] - random value subtracted from another output (added to current)

		reverseIndex := count - i - 1
		v = current - noise[i] + noise[reverseIndex]
		i++
		return satoshi.MustFrom(v)
	})
}

// notSaturatedDistribution - generate NOT-randomized outputs with given constraints:
// 1. first output is less than initialValue
// 2. all other outputs are equal to initialValue
// 3. sum of all outputs = amount
// 4. number of outputs = count
// e.g. For 3 outputs and 8 amount, we have:
// [2, 3, 3]
// WARNING: panics when amount is less than (1 + (count-1) * initialValue)
func (d *ChangeDistribution) notSaturatedDistribution(count uint64, amount satoshi.Value) iter.Seq[satoshi.Value] {
	saturatedOutputs := count - 1
	valueOfSatOuts := satoshi.MustMultiply(saturatedOutputs, d.initialValue)
	if amount > valueOfSatOuts {
		return seq.Concat(
			seq.Of[satoshi.Value](amount-valueOfSatOuts),
			seq.Repeat(satoshi.MustFrom(d.initialValue), saturatedOutputs),
		)
	}

	panic(fmt.Sprintf("Cannot distribute change outputs among given outputs (count: %d) for given amount (%d)", count, amount))
}

// randomNoise randomizes values for each output in the distribution;
// each value is meant to be subtracted from one output and added to another;
// after subtraction, output values are still >= initialValue.
func (d *ChangeDistribution) randomNoise(count uint64, distribution iter.Seq[uint64]) []uint64 {
	noise := make([]uint64, 0, count)
	for current := range distribution {
		randomRange := current - d.initialValue.MustUInt64()
		var randomized uint64
		if randomRange != 0 {
			randomized = d.randomizer(randomRange)
		}
		noise = append(noise, randomized)
	}
	return noise
}
