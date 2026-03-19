package internal

import "github.com/go-softwarelab/common/pkg/types"

type NamedResult[V any] struct {
	types.Result[V]

	name string
}

func NewNamedResult[V any](name string, result *types.Result[V]) *NamedResult[V] {
	return &NamedResult[V]{name: name, Result: *result}
}

func (i *NamedResult[V]) Name() string {
	return i.name
}
