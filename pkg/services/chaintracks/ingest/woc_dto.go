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


func (hdr *WOCBlockHeaderDTO) ToWDK() (*wdk.ChainBlockHeader, error) {
	bitsNum, err := bitsStrToUint32(hdr.Bits)
	if err != nil {
		return nil, fmt.Errorf("invalid bits value %s: %w", hdr.Bits, err)
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

type WOCBlockHeadersDTO []WOCBlockHeaderDTO

func (headers WOCBlockHeadersDTO) ToWDK() ([]*wdk.ChainBlockHeader, error) {
	wdkHeaders := make([]*wdk.ChainBlockHeader, 0, len(headers))
	for _, hdr := range headers {
		wdkHdr, err := hdr.ToWDK()
		if err != nil {
			return nil, fmt.Errorf("failed to convert block header DTO to WDK format: %w", err)
		}
		wdkHeaders = append(wdkHeaders, wdkHdr)
	}
	return wdkHeaders, nil
}
