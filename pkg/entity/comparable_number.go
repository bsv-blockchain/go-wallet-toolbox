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
)

// ComparableNumber represents a numeric value and a comparison operator for flexible numeric filtering and queries.
// The type parameter T must implement types.Number, enabling use with integers or floats in generic operations.
// It is commonly used to specify numeric conditions such as equality, greater than, or less than for queries.
type ComparableNumber[T types.Number] struct {
	Value T
	Cmp   NumberCmpOperator
}
