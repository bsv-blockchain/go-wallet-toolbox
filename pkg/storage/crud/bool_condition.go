package crud

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"

// BoolCondition defines comparison operations for boolean values.
// It enables method chaining for query filters involving bool fields.
type BoolCondition[Parent any] interface {
	Equals(value bool) Parent
	NotEquals(value bool) Parent
	In(value ...bool) Parent
	NotIn(value ...bool) Parent
}

type boolCondition[Parent any] struct {
	parent          Parent
	conditionSetter func(spec *entity.Comparable[bool])
}

func (c *boolCondition[Parent]) Equals(value bool) Parent {
	c.conditionSetter(&entity.Comparable[bool]{
		Value: value,
		Cmp:   entity.Equal,
	})
	return c.parent
}

func (c *boolCondition[Parent]) NotEquals(value bool) Parent {
	c.conditionSetter(&entity.Comparable[bool]{
		Value: value,
		Cmp:   entity.NotEqual,
	})
	return c.parent
}

func (c *boolCondition[Parent]) In(values ...bool) Parent {
	c.conditionSetter(&entity.Comparable[bool]{
		InValues: values,
		Cmp:      entity.In,
	})
	return c.parent
}

func (c *boolCondition[Parent]) NotIn(values ...bool) Parent {
	c.conditionSetter(&entity.Comparable[bool]{
		InValues: values,
		Cmp:      entity.NotIn,
	})
	return c.parent
}
