package repo

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/entity"
	"github.com/go-softwarelab/common/pkg/types"
	"gorm.io/gen"
	"gorm.io/gen/field"
)

type fieldExpr[T any] interface {
	Eq(value T) field.Expr
	Gt(value T) field.Expr
	Lt(value T) field.Expr
	Gte(value T) field.Expr
	Lte(value T) field.Expr
	Neq(value T) field.Expr
}

func cmpCondition[T types.Number](fieldExpr fieldExpr[T], cmp entity.NumberCmpOperator, value T) gen.Condition {
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
	default:
		panic("unsupported comparison operator")
	}
}
