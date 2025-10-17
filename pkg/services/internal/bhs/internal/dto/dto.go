package dto

import (
	"math/big"
	"strings"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type BlockHeaderByHeightResponse struct {
	Hash             string `json:"hash"`
	Version          uint32 `json:"version"`
	PreviousBlock    string `json:"prevBlockHash"`
	MerkleRoot       string `json:"merkleRoot"`
	Timestamp        uint32 `json:"creationTimestamp"`
	DifficultyTarget uint32 `json:"difficultyTarget"`
	Nonce            uint32 `json:"nonce"`
	Work             string `json:"work"`
}

func (b *BlockHeaderByHeightResponse) IsZero() bool { return *b == BlockHeaderByHeightResponse{} }

func (b *BlockHeaderByHeightResponse) ConvertChainBlockHeader() *wdk.ChainBlockHeader {
	return &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
			Version:      b.Version,
			PreviousHash: b.PreviousBlock,
			MerkleRoot:   b.MerkleRoot,
			Time:         b.Timestamp,
			Bits:         b.DifficultyTarget,
			Nonce:        b.Nonce,
		},
		Hash: b.Hash,
	}
}

type TipResponse struct {
	Hash             string   `json:"hash"`
	Version          uint32   `json:"version"`
	PreviousBlock    string   `json:"prevBlockHash"`
	MerkleRoot       string   `json:"merkleRoot"`
	Timestamp        uint32   `json:"creationTimestamp"`
	DifficultyTarget uint32   `json:"difficultyTarget"`
	Nonce            uint32   `json:"nonce"`
	Work             *big.Int `json:"work"`
}

type TipStateResponse struct {
	Header    TipResponse `json:"header"`
	State     string      `json:"state"`
	ChainWork *big.Int    `json:"chainWork"`
	Height    uint        `json:"height"`
}

func (t *TipStateResponse) IsZero() bool { return *t == TipStateResponse{} }

func (t *TipStateResponse) ConvertToChainBlockHeader() *wdk.ChainBlockHeader {
	return &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
			Version:      t.Header.Version,
			PreviousHash: t.Header.PreviousBlock,
			MerkleRoot:   t.Header.MerkleRoot,
			Time:         t.Header.Timestamp,
			Nonce:        t.Header.Nonce,
		},
		Height: t.Height,
		Hash:   t.Header.Hash,
	}
}

type MerkleRootVerifyItem struct {
	BlockHeight uint32 `json:"blockHeight"`
	MerkleRoot  string `json:"merkleRoot"`
}

type merkleRootConfirmationState string

const (
	confirmed      merkleRootConfirmationState = "CONFIRMED"
	unableToVerify merkleRootConfirmationState = "UNABLE_TO_VERIFY"
	invalid        merkleRootConfirmationState = "INVALID"
)

type MerkleRootVerifyResp struct {
	ConfirmationState merkleRootConfirmationState `json:"confirmationState"`
}

func (s merkleRootConfirmationState) IsConfirmed() bool {
	return strings.EqualFold(string(s), string(confirmed))
}
func (s merkleRootConfirmationState) IsInvalid() bool {
	return strings.EqualFold(string(s), string(invalid))
}

func (s merkleRootConfirmationState) IsUnableToVerify() bool {
	return strings.EqualFold(string(s), string(unableToVerify))
}
