package repo

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/go-softwarelab/common/pkg/types"
	"gorm.io/gen"
	"gorm.io/gen/field"
)

type fieldExpr[T types.Ordered] interface {
	Eq(value T) field.Expr
	Gt(value T) field.Expr
	Lt(value T) field.Expr
	Gte(value T) field.Expr
	Lte(value T) field.Expr
	Neq(value T) field.Expr
	Between(left T, right T) field.Expr
	NotBetween(left T, right T) field.Expr
	Like(value T) field.Expr
	NotLike(value T) field.Expr
	In(values ...T) field.Expr
	NotIn(values ...T) field.Expr
}

type comparableExpr[T types.Ordered] interface {
	Comparator() entity.CmpOperator
	GetValue() T
	GetValueRight() T
	GetInValues() []T
}

func cmpCondition[T types.Ordered](fieldExpr fieldExpr[T], cmpExpr comparableExpr[T]) gen.Condition {
	value := cmpExpr.GetValue()
	cmp := cmpExpr.Comparator()

	switch cmp {
	case entity.Equal:
		return fieldExpr.Eq(value)
	case entity.GreaterThan:
		return fieldExpr.Gt(value)
	case entity.LessThan:
		return fieldExpr.Lt(value)
	case entity.GreaterThanOrEqual:
		return fieldExpr.Gte(value)
	case entity.LessThanOrEqual:
		return fieldExpr.Lte(value)
	case entity.NotEqual:
		return fieldExpr.Neq(value)
	case entity.Between:
		return fieldExpr.Between(ordered(value, cmpExpr.GetValueRight()))
	case entity.NotBetween:
		return fieldExpr.NotBetween(ordered(value, cmpExpr.GetValueRight()))
	case entity.Like:
		return fieldExpr.Like(value)
	case entity.NotLike:
		return fieldExpr.NotLike(value)
	case entity.In:
		return fieldExpr.In(cmpExpr.GetInValues()...)
	case entity.NotIn:
		return fieldExpr.NotIn(cmpExpr.GetInValues()...)
	default:
		panic("unsupported comparison operator " + cmp.String())
	}
}

func ordered[T types.Ordered](a, b T) (T, T) {
	if a > b {
		return b, a
	}
	return a, b
}

func cmpBoolCondition(field field.Bool, cmp *entity.Comparable[bool]) gen.Condition {
	switch cmp.Cmp {
	case entity.Equal:
		return field.Is(cmp.Value)
	case entity.NotEqual:
		return field.Is(!cmp.Value)

	case entity.GreaterThan,
		entity.LessThan,
		entity.GreaterThanOrEqual,
		entity.LessThanOrEqual,
		entity.Between,
		entity.NotBetween,
		entity.Like,
		entity.NotLike,
		entity.In,
		entity.NotIn:
		panic("unsupported comparison operator for bool: " + cmp.Cmp.String())

	default:
		panic("unknown comparison operator for bool: " + cmp.Cmp.String())
	}
}

func orderedTime(a, b time.Time) (time.Time, time.Time) {
	if a.After(b) {
		return b, a
	}
	return a, b
}
func cmpTimeCondition(f field.Time, cmp *entity.Comparable[time.Time]) gen.Condition {
	switch cmp.Cmp {
	case entity.Equal:
		return f.Eq(cmp.Value)
	case entity.NotEqual:
		return f.Neq(cmp.Value)
	case entity.GreaterThan:
		return f.Gt(cmp.Value)
	case entity.LessThan:
		return f.Lt(cmp.Value)
	case entity.GreaterThanOrEqual:
		return f.Gte(cmp.Value)
	case entity.LessThanOrEqual:
		return f.Lte(cmp.Value)
	case entity.Between:
		left, right := orderedTime(cmp.GetValue(), cmp.GetValueRight())
		return field.And(f.Gte(left), f.Lte(right))
	case entity.NotBetween:
		left, right := orderedTime(cmp.GetValue(), cmp.GetValueRight())
		return field.Or(f.Lt(left), f.Gt(right))
	case entity.In:
		return f.In(cmp.InValues...)
	case entity.NotIn:
		return f.NotIn(cmp.InValues...)
	case entity.Like, entity.NotLike:
		panic("unsupported comparison operator for time.Time: " + cmp.Cmp.String())
	default:
		panic("unsupported comparison operator for time.Time: " + cmp.Cmp.String())
	}
}
