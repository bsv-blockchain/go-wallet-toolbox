package chaintracks

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"

// InfoResponse contains details about the BSV network, block heights, storage backend, endpoints, and installed packages.
type InfoResponse struct {
	Chain         defs.BSVNetwork `json:"chain"`
	HeightBulk    int             `json:"heightBulk"`
	HeightLive    int             `json:"heightLive"`
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

	// TODO: Below fields are not part of TS-BaseBlockHeader, even though they are returned from server - check if they are always present
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
	Height uint32 `json:"height"`
	Hash   string `json:"hash"`
}

// ResponseFrame represents a generic response wrapper with status information and an optional value payload.
// Used for unmarshalling HTTP API responses where the frame's status field indicates success or error state.
type ResponseFrame[T any] struct {
	Status string `json:"status"` // TODO: Check if other-than-"success" values are possible
	Value  *T     `json:"value,omitempty"`
}

// IsSuccess returns true if the response status is "success"
func (c *ResponseFrame[T]) IsSuccess() bool {
	return c.Status == "success"
}

// IsNotFound returns true if the response status is "success" and the value is nil, indicating a not found result.
// NOTE: Current server implementation returns HTTP 200 with {"status":"success"} for not found cases.
func (c *ResponseFrame[T]) IsNotFound() bool {
	return c.Status == "success" && c.Value == nil
}
