package repo

import (
	"fmt"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
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

func cmpCondition(fieldExpr fieldExpr[uint64], cmp entity.NumberCmpOperator, value uint64) gen.Condition {
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
		panic(fmt.Sprintf("unsupported comparison operator: %s", cmp))
	}
}
