package models

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
)

// InfoResponse contains details about the BSV network, block heights, storage backend, endpoints, and installed packages.
type InfoResponse struct {
	Chain         defs.BSVNetwork `json:"chain"`
	HeightBulk    uint            `json:"heightBulk"`
	HeightLive    uint            `json:"heightLive"`
	Storage       string          `json:"storage"`
	BulkIngestors []string        `json:"bulkIngestors"`
	LiveIngestors []string        `json:"liveIngestors"`
	Packages      []PackageInfo   `json:"packages"`
}

// PackageInfo represents the metadata for an installed package, including its name and version.
type PackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// BaseBlockHeader defines the essential fields for a blockchain block header, including version, hash links, and metadata.
type BaseBlockHeader struct {
	Version      uint32 `json:"version"`
	PreviousHash string `json:"previousHash"`
	MerkleRoot   string `json:"merkleRoot"`
	Time         uint32 `json:"time"`
	Bits         uint32 `json:"bits"`
	Nonce        uint32 `json:"nonce"`
}

// NewBaseBlockHeaderFromWDK creates a BaseBlockHeader from a given wdk.ChainBaseBlockHeader instance.
// The existence of those two types is to decouple the chaintracks service DTOs from the WDK's models.
func NewBaseBlockHeaderFromWDK(bh *wdk.ChainBaseBlockHeader) BaseBlockHeader {
	return BaseBlockHeader{
		Version:      bh.Version,
		PreviousHash: bh.PreviousHash,
		MerkleRoot:   bh.MerkleRoot,
		Time:         bh.Time,
		Bits:         bh.Bits,
		Nonce:        bh.Nonce,
	}
}

// BlockHeaderInfo provides summarized metadata for a block header including identifiers, chainwork, and chain tip status.
type BlockHeaderInfo struct {
	HeaderID         uint   `json:"headerId"`
	PreviousHeaderID *uint  `json:"previousHeaderId,omitempty"`
	Chainwork        string `json:"chainwork"`
	IsChainTip       bool   `json:"isChainTip"`
	IsActive         bool   `json:"isActive"`
}

// BlockHeader represents a full blockchain block header, including base header fields, block height, and block hash.
// It extends BaseBlockHeader with additional height and hash information for block identification and lookup.
type BlockHeader struct {
	BaseBlockHeader
	BlockHeaderInfo

	Height uint32 `json:"height"`
	Hash   string `json:"hash"`
}

// ResponseStatus represents the status of a response, typically indicating success or error.
type ResponseStatus string

// Possible ResponseStatus values
const (
	ResponseStatusSuccess ResponseStatus = "success"
	ResponseStatusError   ResponseStatus = "error"
)

// ResponseFrame represents a generic response wrapper with status information and an optional value payload.
// Used for unmarshalling HTTP API responses where the frame's status field indicates success or error state.
type ResponseFrame[T any] struct {
	Status ResponseStatus `json:"status"` // TODO: Check if other-than-"success" values are possible
	Value  *T             `json:"value,omitempty"`
}

// IsSuccess returns true if the response status is "success"
func (c *ResponseFrame[T]) IsSuccess() bool {
	return c.Status == ResponseStatusSuccess
}

// IsNotFound returns true if the response status is "success" and the value is nil, indicating a not found result.
// NOTE: Current server implementation returns HTTP 200 with {"status":"success"} for not found cases.
func (c *ResponseFrame[T]) IsNotFound() bool {
	return c.Status == ResponseStatusSuccess && c.Value == nil
}

// HashedBaseHeaders represents a hex-encoded string of one or more concatenated Bitcoin base block headers.
// Used for efficient transport of serialized block header data in API responses or storage contexts.
// To obtain individual block headers, methods should decode and parse this type appropriately.
// An empty value denotes no serialized block headers are present.
type HashedBaseHeaders string

// ToBaseBlockHeaders decodes the hex-encoded string and returns a slice of BaseBlockHeader pointers or an error.
// Returns an error if the string is not valid hex or its length is not a multiple of the block header length (80 bytes).
// Each decoded header is parsed into a BaseBlockHeader struct and included in the result slice.
// If the input is an empty string, an empty slice is returned without error.
func (h HashedBaseHeaders) ToBaseBlockHeaders() ([]*BaseBlockHeader, error) {
	data, err := hex.DecodeString(string(h))
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex string: %w", err)
	}

	const headerLength = wdk.BlockHeaderLength
	if len(data)%headerLength != 0 {
		return nil, fmt.Errorf("decoded data length %d is not a multiple of header length %d", len(data), headerLength)
	}

	numHeaders := len(data) / headerLength
	hashes := make([]*BaseBlockHeader, numHeaders)
	for i := 0; i < numHeaders; i++ {
		start := i * headerLength
		end := start + headerLength
		chainBaseHeader, err := wdk.ChainBaseBlockHeaderFromBytes(data[start:end])
		if err != nil {
			return nil, fmt.Errorf("failed to parse block header at index %d: %w", i, err)
		}

		hashes[i] = to.Ptr(NewBaseBlockHeaderFromWDK(chainBaseHeader))
	}

	return hashes, nil
}

// FiatExchangeRates represents fiat currency exchange rate data at a specific timestamp.
// It includes the base currency, a mapping of currency codes to rates, and the time of the rates' validity.
type FiatExchangeRates struct {
	Timestamp time.Time          `json:"timestamp"`
	Rates     map[string]float64 `json:"rates"`
	Base      string             `json:"base"`
}
