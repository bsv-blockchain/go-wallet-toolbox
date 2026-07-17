package funder

import (
	"sort"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	tierMined    = 0
	tierUnproven = 1
	tierSending  = 2
	tierCount    = 3
)

// utxoPool partitions UTXOs by status tier (mined first, then unproven, then sending).
// It is used only by the sweep path, which allocates every UTXO via all(); the
// non-sweep path selects UTXOs with bounded target-aware repository queries instead
// (see SQL.allocateBounded).
type utxoPool struct {
	tiers [tierCount][]*models.UserUTXO
}

func newUTXOPool(utxos []*models.UserUTXO) *utxoPool {
	p := &utxoPool{}
	for _, u := range utxos {
		tier := statusToTier(u.UTXOStatus)
		p.tiers[tier] = append(p.tiers[tier], u)
	}
	for i := range p.tiers {
		sort.Slice(p.tiers[i], func(a, b int) bool {
			return p.tiers[i][a].Satoshis < p.tiers[i][b].Satoshis
		})
	}
	return p
}

// all returns every UTXO across all tiers (for sweep mode).
func (p *utxoPool) all() []*models.UserUTXO {
	var result []*models.UserUTXO
	for _, tier := range p.tiers {
		result = append(result, tier...)
	}
	return result
}

func statusToTier(status wdk.UTXOStatus) int {
	switch status {
	case wdk.UTXOStatusMined:
		return tierMined
	case wdk.UTXOStatusUnproven:
		return tierUnproven
	case wdk.UTXOStatusSending:
		return tierSending
	default:
		return tierSending
	}
}
