package servicequeue

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal"
	"github.com/go-softwarelab/common/pkg/types"
)

type NamedResult[R any] = internal.NamedResult[R]

func NewNamedResult[R any](name string, result *types.Result[R]) *NamedResult[R] {
	//nolint:unconvert // cannot skip the conversion, because compiler is failing
	return (*NamedResult[R])(internal.NewNamedResult(name, result))
}
