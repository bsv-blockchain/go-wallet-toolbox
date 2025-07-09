package entity

import "github.com/go-softwarelab/common/pkg/types"

// NumberCmpOperator defines comparison operations for numeric types such as equality, inequality, and range-based checks.
type NumberCmpOperator int

// Possible values for NumberCmpOperator, representing different comparison operations.
const (
	GreaterThan NumberCmpOperator = iota
	LessThan
	Equal
	NotEqual
	GreaterThanOrEqual
	LessThanOrEqual
	Between
	NotBetween
)

// ComparableNumber represents a numeric value and a comparison operator for flexible numeric filtering and queries.
// The type parameter T must implement types.Number, enabling use with integers or floats in generic operations.
// It is commonly used to specify numeric conditions such as equality, greater than, or less than for queries.
type ComparableNumber[T types.Number] struct {
	Value  T
	Value2 T
	Cmp    NumberCmpOperator
}

// Comparator returns the current NumberCmpOperator that specifies the comparison logic for the ComparableNumber instance.
func (c *ComparableNumber[T]) Comparator() NumberCmpOperator {
	return c.Cmp
}

// GetValue returns the primary numeric value held by the ComparableNumber for use in comparisons or queries.
func (c *ComparableNumber[T]) GetValue() T {
	return c.Value
}

// GetValue2 returns the secondary numeric value stored in the ComparableNumber, primarily used for range comparisons (e.g., Between).
func (c *ComparableNumber[T]) GetValue2() T {
	return c.Value2
}
