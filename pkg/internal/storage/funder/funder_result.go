package funder

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/go-softwarelab/common/pkg/seq"
)

type Result struct {
	AllocatedUTXOs     []*UTXO
	ChangeOutputsCount uint64
	ChangeAmount       satoshi.Value
	Fee                satoshi.Value
}

func (fr *Result) TotalAllocated() (satoshi.Value, error) {
	total, err := satoshi.Sum(seq.Map(seq.FromSlice(fr.AllocatedUTXOs), func(utxo *UTXO) satoshi.Value {
		return utxo.Satoshis
	}))
	if err != nil {
		return 0, fmt.Errorf("failed to sum allocated UTXOs: %w", err)
	}

	return total, nil
}

type UTXO struct {
	OutputID uint
	Satoshis satoshi.Value
}
