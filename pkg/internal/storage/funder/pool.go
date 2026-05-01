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

// utxoPool partitions UTXOs by status tier and supports 3-stage best-fit selection:
// exact match → smallest sufficient → largest insufficient, evaluated tier by tier
// (mined first, then unproven, then sending).
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

// selectBest finds the optimal UTXO for the given target using 3-stage selection
// across all tiers (safest first):
//  1. Exact match (satoshis == target)
//  2. Smallest sufficient (smallest satoshis >= target)
//  3. Largest insufficient (largest satoshis < target)
func (p *utxoPool) selectBest(targetSatoshis uint64) *models.UserUTXO {
	for tierIdx := range p.tiers {
		if len(p.tiers[tierIdx]) == 0 {
			continue
		}

		// Stage 1: Exact match
		for i, u := range p.tiers[tierIdx] {
			if u.Satoshis == targetSatoshis {
				return p.removeAt(tierIdx, i)
			}
		}

		// Stage 2: Smallest sufficient (tier sorted ASC, first >= target)
		for i, u := range p.tiers[tierIdx] {
			if u.Satoshis >= targetSatoshis {
				return p.removeAt(tierIdx, i)
			}
		}

		// Stage 3: Largest insufficient (last element in ASC-sorted tier)
		last := len(p.tiers[tierIdx]) - 1
		return p.removeAt(tierIdx, last)
	}
	return nil
}

// all returns every remaining UTXO across all tiers (for sweep mode).
func (p *utxoPool) all() []*models.UserUTXO {
	var result []*models.UserUTXO
	for _, tier := range p.tiers {
		result = append(result, tier...)
	}
	return result
}

func (p *utxoPool) removeAt(tierIdx, i int) *models.UserUTXO {
	tier := p.tiers[tierIdx]
	u := tier[i]
	p.tiers[tierIdx] = append(tier[:i], tier[i+1:]...)
	return u
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
