package entity

import "github.com/go-softwarelab/common/pkg/types"

// CmpOperator defines an integer-based enumeration representing various comparison operators
type CmpOperator int

// Possible values for CmpOperator, representing different comparison operations.
const (
	GreaterThan CmpOperator = iota
	LessThan
	Equal
	NotEqual
	GreaterThanOrEqual
	LessThanOrEqual
	Between
	NotBetween
	Like
	NotLike
	In
	NotIn
)

// Comparable represents a generic comparison filter with optional range capability and a specified comparison operator.
// It stores two values and a CmpOperator to define various query or matching conditions for supported comparable types.
type Comparable[T types.Comparable] struct {
	Value      T
	ValueRight T   // Used for range comparisons, e.g., Between
	InValues   []T // Used for In/NotIn comparisons
	Cmp        CmpOperator
}

// Comparator returns the current CmpOperator that specifies the comparison logic for the Comparable instance.
func (c *Comparable[T]) Comparator() CmpOperator {
	return c.Cmp
}

// GetValue returns the primary numeric value held by the Comparable for use in comparisons or queries.
func (c *Comparable[T]) GetValue() T {
	return c.Value
}

// GetValueRight returns the secondary numeric value stored in the Comparable, primarily used for range comparisons (e.g., Between).
func (c *Comparable[T]) GetValueRight() T {
	return c.ValueRight
}

// GetInValues returns a slice of values used for In/NotIn comparisons, allowing for multiple values to be checked against.
func (c *Comparable[T]) GetInValues() []T {
	return c.InValues
}
