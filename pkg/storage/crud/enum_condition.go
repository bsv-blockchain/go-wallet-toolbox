package crud

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"

// StringEnumCondition defines composable filter operations for string-based enum fields in query building.
// It provides equality, inequality and pattern-matching methods that return the Parent type for fluent chainability.
// Methods enable use of equals, not-equals, like, not-like, in, and not-in filters for enum-typed columns.
type StringEnumCondition[Parent any, EnumType ~string] interface {
	Equals(value EnumType) Parent
	NotEquals(value EnumType) Parent
	Like(value EnumType) Parent
	NotLike(value EnumType) Parent
	In(value ...EnumType) Parent
	NotIn(value ...EnumType) Parent
}

type stringEnumCondition[Parent any, EnumType ~string] struct {
	parent          Parent
	conditionSetter func(spec *entity.Comparable[EnumType])
}

func (c *stringEnumCondition[Parent, EnumType]) Equals(value EnumType) Parent {
	c.conditionSetter(&entity.Comparable[EnumType]{
		Value: value,
		Cmp:   entity.Equal,
	})

	return c.parent
}

func (c *stringEnumCondition[Parent, EnumType]) NotEquals(value EnumType) Parent {
	c.conditionSetter(&entity.Comparable[EnumType]{
		Value: value,
		Cmp:   entity.NotEqual,
	})

	return c.parent
}

func (c *stringEnumCondition[Parent, EnumType]) Like(value EnumType) Parent {
	c.conditionSetter(&entity.Comparable[EnumType]{
		Value: value,
		Cmp:   entity.Like,
	})

	return c.parent
}

func (c *stringEnumCondition[Parent, EnumType]) NotLike(value EnumType) Parent {
	c.conditionSetter(&entity.Comparable[EnumType]{
		Value: value,
		Cmp:   entity.NotLike,
	})

	return c.parent
}

func (c *stringEnumCondition[Parent, EnumType]) In(value ...EnumType) Parent {
	c.conditionSetter(&entity.Comparable[EnumType]{
		InValues: value,
		Cmp:      entity.In,
	})

	return c.parent
}

func (c *stringEnumCondition[Parent, EnumType]) NotIn(values ...EnumType) Parent {
	c.conditionSetter(&entity.Comparable[EnumType]{
		InValues: values,
		Cmp:      entity.NotIn,
	})

	return c.parent
}
