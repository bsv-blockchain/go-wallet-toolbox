package ingest

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"time"

	crypto "github.com/bsv-blockchain/go-sdk/primitives/hash"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
)

// BulkHeaderFilesInfo represents metadata about a collection of bulk block header files and their containing folder.
type BulkHeaderFilesInfo struct {
	RootFolder     string               `json:"rootFolder"`
	JSONFilename   string               `json:"jsonFilename"`
	HeadersPerFile int                  `json:"headersPerFile"`
	Files          []BulkHeaderFileInfo `json:"files"`
}

type BulkHeaderMinimumInfo struct {
	FirstHeight uint   `json:"firstHeight"`
	Count       int    `json:"count"`
	FileName    string `json:"fileName"`
	SourceURL   string `json:"sourceUrl"`
}

// ToHeightRange returns the HeightRange spanned by this BulkHeaderFileInfo based on its FirstHeight and Count fields.
func (b *BulkHeaderMinimumInfo) ToHeightRange() models.HeightRange {
	return models.NewHeightRange(b.FirstHeight, b.FirstHeight+must.ConvertToUInt(b.Count)-1)
}

// Validate checks bulk file data for validity including hash, length, and count consistency with its metadata.
// Returns an error if the data is empty, hash mismatches, length is not a multiple of 80, or count is inconsistent.
func (b *BulkHeaderMinimumInfo) Validate(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("bulk file data is empty")
	}

	if len(data)%80 != 0 {
		return fmt.Errorf("bulk file data length is not a multiple of 80 bytes: got %d bytes", len(data))
	}

	if len(data)/80 != b.Count {
		return fmt.Errorf("bulk file data header count mismatch: expected %d, got %d", b.Count, len(data)/80)
	}

	return nil
}

// BulkHeaderFileInfo contains metadata related to a single bulk block header file for a specific blockchain network.
type BulkHeaderFileInfo struct {
	BulkHeaderMinimumInfo `json:",inline"`
	PrevChainWork         string          `json:"prevChainWork"`
	LastChainWork         string          `json:"lastChainWork"`
	PrevHash              string          `json:"prevHash"`
	LastHash              *string         `json:"lastHash,omitempty"`
	FileHash              []byte          `json:"fileHash,omitempty"`
	Chain                 defs.BSVNetwork `json:"chain,omitempty"`
}

// Equals compares two BulkHeaderFileInfo instances and returns true if they represent the same header file metadata.
func (b *BulkHeaderFileInfo) Equals(other *BulkHeaderFileInfo) bool {
	return b != nil && other != nil &&
		b.FirstHeight == other.FirstHeight &&
		b.Count == other.Count &&
		((b.LastHash == nil && other.LastHash == nil) || (b.LastHash != nil && other.LastHash != nil && *b.LastHash == *other.LastHash)) &&
		b.LastChainWork == other.LastChainWork &&
		bytes.Equal(b.FileHash, other.FileHash) &&
		b.Chain == other.Chain

}

// BulkFileData represents a complete bulk block header file and its metadata for a specific blockchain network.
type BulkFileData struct {
	Info      BulkHeaderFileInfo
	Data      []byte
	UpdatedAt time.Time
}

// Validate checks bulk file data for validity including hash, length, and count consistency with its metadata.
// Returns an error if the data is empty, hash mismatches, length is not a multiple of 80, or count is inconsistent.
func (b *BulkFileData) Validate() error {
	if err := b.Info.Validate(b.Data); err != nil {
		return err
	}

	dataHash := b.Hash()
	if !bytes.Equal(dataHash, b.Info.FileHash) {
		base64Expected := base64.StdEncoding.EncodeToString(b.Info.FileHash)
		base64Got := base64.StdEncoding.EncodeToString(dataHash)
		return fmt.Errorf("bulk file data hash mismatch: expected %s, got %s", base64Expected, base64Got)
	}

	return nil
}

// Hash computes and returns the SHA-256 hash of the bulk file data bytes as a byte slice.
func (b *BulkFileData) Hash() []byte {
	return crypto.Sha256(b.Data)
}

// Len returns the number of block headers contained in the bulk file data.
func (b *BulkFileData) Len() int {
	return len(b.Data) / 80
}

// GetHeaderDataAtIndex returns the 80-byte raw header data at the specified index within the bulk file data.
// Returns an error if the index is out of bounds or invalid.
func (b *BulkFileData) GetHeaderDataAtIndex(index uint) ([]byte, error) {
	return GetHeaderDataAtIndex(b.Data, index)
}

// GetHeaderAtIndex returns the decoded block header at the specified index from the bulk file data.
// Returns an error if the index is out of bounds or if header parsing fails.
func (b *BulkFileData) GetHeaderAtIndex(index uint) (*wdk.ChainBlockHeader, error) {
	return GetHeaderAtIndex(b.Data, index, b.Info.FirstHeight)
}

// GetHeaderDataAtIndex returns the 80-byte raw header data at the specified index within the bulk file data.
// Returns an error if the index is out of bounds or invalid.
func GetHeaderDataAtIndex(data []byte, index uint) ([]byte, error) {
	length := len(data) / 80
	if index >= must.ConvertToUInt(length) {
		return nil, fmt.Errorf("index %d out of bounds for bulk file data with %d headers", index, length)
	}

	start := index * 80
	end := start + 80
	return data[start:end], nil
}

// GetHeaderAtIndex returns the decoded block header at the specified index from the bulk file data.
// Returns an error if the index is out of bounds or if header parsing fails.
func GetHeaderAtIndex(data []byte, index, firstHeight uint) (*wdk.ChainBlockHeader, error) {
	headerData, err := GetHeaderDataAtIndex(data, index)
	if err != nil {
		return nil, err
	}

	baseBlockHeader, err := wdk.ChainBaseBlockHeaderFromBytes(headerData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse block header at index %d: %w", index, err)
	}

	blockHash, err := baseBlockHeader.CalculateHash()
	if err != nil {
		return nil, fmt.Errorf("failed to compute hash for block header at index %d: %w", index, err)
	}

	header := &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: *baseBlockHeader,
		Height:               firstHeight + must.ConvertToUInt(index),
		Hash:                 blockHash.String(),
	}

	return header, nil
}
