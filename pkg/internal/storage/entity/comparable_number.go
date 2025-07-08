package entity

import "github.com/go-softwarelab/common/pkg/types"

type NumberCmpOperator string

const (
	GreaterThan        NumberCmpOperator = ">"
	LessThan           NumberCmpOperator = "<"
	Equal              NumberCmpOperator = "="
	NotEqual           NumberCmpOperator = "!="
	GreaterThanOrEqual NumberCmpOperator = ">="
	LessThanOrEqual    NumberCmpOperator = "<="
)

type ComparableNumber[T types.Number] struct {
	Value T
	Cmp   NumberCmpOperator
}
