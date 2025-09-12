package entity

import (
	"fmt"

	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/types"
)

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

// ToStringComparable converts a Comparable of any comparable type to a Comparable of string type using fmt.Sprint for values.
func (c *Comparable[T]) ToStringComparable() *Comparable[string] {
	return &Comparable[string]{
		Value:      fmt.Sprint(c.Value),
		ValueRight: fmt.Sprint(c.ValueRight),
		InValues:   slices.Map(c.InValues, func(v T) string { return fmt.Sprint(v) }),
		Cmp:        c.Cmp,
	}
}

func (op CmpOperator) String() string {
	switch op {
	case GreaterThan:
		return "gt"
	case LessThan:
		return "lt"
	case Equal:
		return "eq"
	case NotEqual:
		return "neq"
	case GreaterThanOrEqual:
		return "gte"
	case LessThanOrEqual:
		return "lte"
	case Between:
		return "between"
	case NotBetween:
		return "not_between"
	case Like:
		return "like"
	case NotLike:
		return "not_like"
	case In:
		return "in"
	case NotIn:
		return "not_in"
	default:
		return "unknown"
	}
}

// ComparableSet represents a set-based comparison filter for types that are comparable.
type ComparableSet[T comparable] struct {
	ContainAny []T
	ContainAll []T
	Empty      bool
}
