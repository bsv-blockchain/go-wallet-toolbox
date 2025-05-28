package entity

import "github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"

type ProvenTxToSync struct {
	TxID     string
	Attempts uint64
	Status   wdk.ProvenTxReqStatus
}

type ProvenTxAsMined struct {
	TxID        string
	BlockHeight uint32
	MerklePath  []byte
	MerkleRoot  string
	BlockHash   string
}
