package crud

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/entity"
	"github.com/go-softwarelab/common/pkg/types"
)

// NumericCondition defines numeric comparison operations for commission queries using a generic type T.
// It allows chaining equality, greater-than, less-than, and related numeric conditions for filtering results.
type NumericCondition[Parent any, T types.Number] interface {
	Equals(value T) Parent
	GreaterThan(value T) Parent
	LessThan(value T) Parent
	GreaterThanOrEqual(value T) Parent
	LessThanOrEqual(value T) Parent
	Between(left, right T) Parent
	NotBetween(left, right T) Parent
}

type numericCondition[Parent any, T types.Number] struct {
	parent          Parent
	conditionSetter func(spec *entity.ComparableNumber[T])
}

func (c *numericCondition[Parent, T]) Equals(value T) Parent {
	c.conditionSetter(&entity.ComparableNumber[T]{
		Value: value,
		Cmp:   entity.Equal,
	})

	return c.parent
}

func (c *numericCondition[Parent, T]) GreaterThan(value T) Parent {
	c.conditionSetter(&entity.ComparableNumber[T]{
		Value: value,
		Cmp:   entity.GreaterThan,
	})

	return c.parent
}

func (c *numericCondition[Parent, T]) LessThan(value T) Parent {
	c.conditionSetter(&entity.ComparableNumber[T]{
		Value: value,
		Cmp:   entity.LessThan,
	})

	return c.parent
}

func (c *numericCondition[Parent, T]) GreaterThanOrEqual(value T) Parent {
	c.conditionSetter(&entity.ComparableNumber[T]{
		Value: value,
		Cmp:   entity.GreaterThanOrEqual,
	})

	return c.parent
}

func (c *numericCondition[Parent, T]) LessThanOrEqual(value T) Parent {
	c.conditionSetter(&entity.ComparableNumber[T]{
		Value: value,
		Cmp:   entity.LessThanOrEqual,
	})

	return c.parent
}

func (c *numericCondition[Parent, T]) Between(left, right T) Parent {
	c.conditionSetter(&entity.ComparableNumber[T]{
		Value:  left,
		Value2: right,
		Cmp:    entity.Between,
	})

	return c.parent
}

func (c *numericCondition[Parent, T]) NotBetween(left, right T) Parent {
	c.conditionSetter(&entity.ComparableNumber[T]{
		Value:  left,
		Value2: right,
		Cmp:    entity.NotBetween,
	})

	return c.parent
}
