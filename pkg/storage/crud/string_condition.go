package crud

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"

// StringCondition models string-based filtering operations for building composable query conditions.
// It provides equality, inequality, and pattern-matching methods that return the Parent type for fluent chaining.
type StringCondition[Parent any] interface {
	Equals(value string) Parent
	NotEquals(value string) Parent
	Like(value string) Parent
	NotLike(value string) Parent
	In(value ...string) Parent
	NotIn(value ...string) Parent
}

type stringCondition[Parent any] struct {
	parent          Parent
	conditionSetter func(spec *entity.Comparable[string])
}

func (c *stringCondition[Parent]) Equals(value string) Parent {
	c.conditionSetter(&entity.Comparable[string]{
		Value: value,
		Cmp:   entity.Equal,
	})

	return c.parent
}

func (c *stringCondition[Parent]) NotEquals(value string) Parent {
	c.conditionSetter(&entity.Comparable[string]{
		Value: value,
		Cmp:   entity.NotEqual,
	})

	return c.parent
}

func (c *stringCondition[Parent]) Like(value string) Parent {
	c.conditionSetter(&entity.Comparable[string]{
		Value: value,
		Cmp:   entity.Like,
	})

	return c.parent
}

func (c *stringCondition[Parent]) NotLike(value string) Parent {
	c.conditionSetter(&entity.Comparable[string]{
		Value: value,
		Cmp:   entity.NotLike,
	})

	return c.parent
}

func (c *stringCondition[Parent]) In(value ...string) Parent {
	c.conditionSetter(&entity.Comparable[string]{
		InValues: value,
		Cmp:      entity.In,
	})

	return c.parent
}

func (c *stringCondition[Parent]) NotIn(value ...string) Parent {
	c.conditionSetter(&entity.Comparable[string]{
		InValues: value,
		Cmp:      entity.NotIn,
	})

	return c.parent
}
