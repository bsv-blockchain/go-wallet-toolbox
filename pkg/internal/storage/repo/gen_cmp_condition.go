package repo

import (
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
}

type comparableExpr[T types.Ordered] interface {
	Comparator() entity.CmpOperator
	GetValue() T
	GetValue2() T
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
		return fieldExpr.Between(ordered(value, cmpExpr.GetValue2()))
	case entity.NotBetween:
		return fieldExpr.NotBetween(ordered(value, cmpExpr.GetValue2()))
	default:
		panic("unsupported comparison operator")
	}
}

func ordered[T types.Ordered](a, b T) (T, T) {
	if a > b {
		return b, a
	}
	return a, b
}
