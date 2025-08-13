package bitails

import (
	"encoding/binary"
	"fmt"
	"net/url"
	"path"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/to"
)

// ConvertHeader decodes an 80-byte raw header and fills all fields.
func ConvertHeader(raw []byte, height uint32) (*wdk.ChainBlockHeader, error) {
	const (
		versionOffset = 0
		versionSize   = 4

		prevHashOffset = versionOffset + versionSize
		prevHashLength = MerkleRootOffset - prevHashOffset

		timeOffset  = MerkleRootOffset + MerkleRootLength
		bitsOffset  = timeOffset + versionSize
		nonceOffset = bitsOffset + versionSize
	)

	if len(raw) != BlockHeaderLength {
		return nil, fmt.Errorf("ConvertHeader: want %d bytes, got %d",
			BlockHeaderLength, len(raw))
	}

	readLE32 := func(off int) uint32 {
		return binary.LittleEndian.Uint32(raw[off : off+4])
	}

	version := readLE32(versionOffset)

	var prevHash, merkleRoot chainhash.Hash
	copy(prevHash[:], raw[prevHashOffset:prevHashOffset+prevHashLength])
	copy(merkleRoot[:], raw[MerkleRootOffset:MerkleRootOffset+MerkleRootLength])

	timestamp := readLE32(timeOffset)
	bits := readLE32(bitsOffset)
	nonce := readLE32(nonceOffset)

	blockHash := chainhash.DoubleHashH(raw).String()

	return &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
			Version:      version,
			PreviousHash: prevHash.String(),
			MerkleRoot:   merkleRoot.String(),
			Time:         timestamp,
			Bits:         bits,
			Nonce:        nonce,
		},
		Height: uint(height),
		Hash:   blockHash,
	}, nil
}

func validateScriptHash(scriptHash string) error {
	if scriptHash == "" {
		return fmt.Errorf("scripthash cannot be empty")
	}

	if len(scriptHash) != 64 {
		return fmt.Errorf("invalid scripthash length: must be 64 characters for a SHA256 hash")
	}

	if err := primitives.HexString(scriptHash).Validate(); err != nil {
		return fmt.Errorf("invalid scripthash format: %w", err)
	}

	return nil
}

func calcDepth(tip, height uint32) (*int, error) {
	if height > tip {
		return nil, fmt.Errorf("invalid block height: height=%d is greater than tip=%d", height, tip)
	}

	d := int(tip) - int(height) + 1
	return to.Ptr(d), nil
}

// buildURL joins baseURL with any number of path segments, preserving the
func buildURL(baseURL string, segments ...string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL %q: %w", baseURL, err)
	}
	relativePath := path.Join(segments...)
	u = u.ResolveReference(&url.URL{Path: relativePath})
	return u.String(), nil
}

// /tx/{txid}/status
func txStatusURL(baseURL, txID string) (string, error) {
	return buildURL(baseURL, "tx", txID, "status")
}

// /tx/{txid}/proof/tsc
func tscProofURL(baseURL, txID string) (string, error) {
	return buildURL(baseURL, "tx", txID, "proof", "tsc")
}

// /block/{blockHash}/header
func blockHeaderURL(baseURL, blockHash string) (string, error) {
	return buildURL(baseURL, "block", blockHash, "header")
}

// /tx/broadcast/multi
func broadcastURL(baseURL string) (string, error) {
	return buildURL(baseURL, "tx", "broadcast", "multi")
}

// /block/header/height/{blockheight}
func blockByHeight(baseURL string, height uint32) (string, error) {
	return buildURL(baseURL, "block", "height", fmt.Sprintf("%d", height))
}

// /block/header/height/{blockheight}/raw
func blockHeaderByHeightURL(baseURL string, height uint32) (string, error) {
	return buildURL(baseURL, "block", "header", "height", fmt.Sprintf("%d", height), "raw")
}

// /block/latest
func latestBlockURL(baseURL string) (string, error) {
	return buildURL(baseURL, "block", "latest")
}

// /download/tx/{txid}/hex
func rawTxURL(baseURL, txID string) (string, error) {
	return buildURL(baseURL, "download", "tx", txID, "hex")
}

// /scripthash/{scripthash}/history
func scriptHashHistoryURL(baseURL, scriptHash string) (string, error) {
	return buildURL(baseURL, "scripthash", scriptHash, "history")
}
