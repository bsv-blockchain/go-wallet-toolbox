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

// ResponseFrame represents a generic response wrapper with status information and an optional value payload.
// Used for unmarshalling HTTP API responses where the frame's status field indicates success or error state.
type ResponseFrame[T any] struct {
	Status string `json:"status"` // TODO: Check if other-than-"success" values are possible
	Value  *T     `json:"value,omitempty"`
}

// IsSuccess returns true if the response status is "success" and a non-nil value payload is present.
func (c *ResponseFrame[T]) IsSuccess() bool {
	return c.Status == "success" && c.Value != nil
}
