package entity

import (
	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
)

type UpsertOutputForSync struct {
	pkgentity.Output
	BasketNumID *uint
}
