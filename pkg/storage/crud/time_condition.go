package crud

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
)

// TimeCondition defines comparison operations for time.Time fields.
type TimeCondition[Parent any] interface {
	Equals(value time.Time) Parent
	NotEquals(value time.Time) Parent
	GreaterThan(value time.Time) Parent
	GreaterThanOrEqual(value time.Time) Parent
	LessThan(value time.Time) Parent
	LessThanOrEqual(value time.Time) Parent
	Between(start, end time.Time) Parent
	NotBetween(start, end time.Time) Parent
	In(values ...time.Time) Parent
	NotIn(values ...time.Time) Parent
}

type timeCondition[Parent any] struct {
	parent          Parent
	conditionSetter func(spec *entity.Comparable[time.Time])
}

func (c *timeCondition[Parent]) Equals(value time.Time) Parent {
	c.conditionSetter(&entity.Comparable[time.Time]{Value: value, Cmp: entity.Equal})
	return c.parent
}

func (c *timeCondition[Parent]) NotEquals(value time.Time) Parent {
	c.conditionSetter(&entity.Comparable[time.Time]{Value: value, Cmp: entity.NotEqual})
	return c.parent
}

func (c *timeCondition[Parent]) GreaterThan(value time.Time) Parent {
	c.conditionSetter(&entity.Comparable[time.Time]{Value: value, Cmp: entity.GreaterThan})
	return c.parent
}

func (c *timeCondition[Parent]) GreaterThanOrEqual(value time.Time) Parent {
	c.conditionSetter(&entity.Comparable[time.Time]{Value: value, Cmp: entity.GreaterThanOrEqual})
	return c.parent
}

func (c *timeCondition[Parent]) LessThan(value time.Time) Parent {
	c.conditionSetter(&entity.Comparable[time.Time]{Value: value, Cmp: entity.LessThan})
	return c.parent
}

func (c *timeCondition[Parent]) LessThanOrEqual(value time.Time) Parent {
	c.conditionSetter(&entity.Comparable[time.Time]{Value: value, Cmp: entity.LessThanOrEqual})
	return c.parent
}

func (c *timeCondition[Parent]) Between(start, end time.Time) Parent {
	c.conditionSetter(&entity.Comparable[time.Time]{Value: start, ValueRight: end, Cmp: entity.Between})
	return c.parent
}

func (c *timeCondition[Parent]) NotBetween(start, end time.Time) Parent {
	c.conditionSetter(&entity.Comparable[time.Time]{Value: start, ValueRight: end, Cmp: entity.NotBetween})
	return c.parent
}

func (c *timeCondition[Parent]) In(values ...time.Time) Parent {
	c.conditionSetter(&entity.Comparable[time.Time]{InValues: values, Cmp: entity.In})
	return c.parent
}

func (c *timeCondition[Parent]) NotIn(values ...time.Time) Parent {
	c.conditionSetter(&entity.Comparable[time.Time]{InValues: values, Cmp: entity.NotIn})
	return c.parent
}
