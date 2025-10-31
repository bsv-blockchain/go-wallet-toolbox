package ingest

import (
	"fmt"
	"strconv"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// WOCBlockHeaderDTO represents a block header as retrieved from the Whatsonchain API for a given block hash.
type WOCBlockHeaderDTO struct {
	Hash          string  `json:"hash"`
	Size          int     `json:"size"`
	Height        uint    `json:"height"`
	Version       uint32  `json:"version"`
	VersionHex    string  `json:"versionHex"`
	MerkleRoot    string  `json:"merkleroot"`
	Time          uint32  `json:"time"`
	MedianTime    uint32  `json:"mediantime"`
	Nonce         uint32  `json:"nonce"`
	Bits          string  `json:"bits"`
	Difficulty    float64 `json:"difficulty"`
	Chainwork     string  `json:"chainwork"`
	PrevBlock     string  `json:"previousblockhash,omitempty"`
	NextBlock     string  `json:"nextblockhash,omitempty"`
	Confirmations int     `json:"confirmations"`
}

func bitsStrToUint32(bitsStr string) (uint32, error) {
	bitsNum, err := strconv.ParseUint(bitsStr, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid bits value %s: %w", bitsStr, err)
	}

	return uint32(bitsNum), nil
}

const (
	genesisAsPrevBlockHash = "0000000000000000000000000000000000000000000000000000000000000000"
)

// ToWDK converts the WOCBlockHeaderDTO to a wdk.ChainBlockHeader, parsing and validating the Bits field.
// Returns an error if Bits cannot be parsed or another conversion error occurs.
func (hdr *WOCBlockHeaderDTO) ToWDK() (*wdk.ChainBlockHeader, error) {
	bitsNum, err := bitsStrToUint32(hdr.Bits)
	if err != nil {
		return nil, err
	}

	if hdr.PrevBlock == "" {
		hdr.PrevBlock = genesisAsPrevBlockHash
	}

	return &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
			Version:      hdr.Version,
			PreviousHash: hdr.PrevBlock,
			MerkleRoot:   hdr.MerkleRoot,
			Time:         hdr.Time,
			Bits:         bitsNum,
			Nonce:        hdr.Nonce,
		},
		Hash:   hdr.Hash,
		Height: hdr.Height,
	}, nil
}

// WOCBlockHeadersDTO represents a slice of block header DTOs as returned by the WhatsOnChain API for blocks queries.
type WOCBlockHeadersDTO []WOCBlockHeaderDTO

// ToWDK converts the WOCBlockHeadersDTO slice to a slice of wdk.ChainBlockHeader pointers, returning an error on failure.
func (headers WOCBlockHeadersDTO) ToWDK() ([]*wdk.ChainBlockHeader, error) {
	wdkHeaders := make([]*wdk.ChainBlockHeader, 0, len(headers))
	for _, hdr := range headers {
		wdkHdr, err := hdr.ToWDK()
		if err != nil {
			return nil, fmt.Errorf("failed to convert block header %q DTO to WDK format: %w", hdr.Hash, err)
		}
		wdkHeaders = append(wdkHeaders, wdkHdr)
	}
	return wdkHeaders, nil
}
