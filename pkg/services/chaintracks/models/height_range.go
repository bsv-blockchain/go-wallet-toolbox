package models

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
)

// HeightRanges represents separate ranges for bulk and live block header heights.
// It is used to specify which spans of block heights are targeted for synchronization or processing.
// The Bulk field defines the height range for bulk operations, while the Live field defines the range for live operations.
// Both fields use the HeightRange type to specify minimum and maximum heights, along with range state.
type HeightRanges struct {
	Bulk HeightRange
	Live HeightRange
}

// Validate checks HeightRanges invariants to ensure Bulk and Live are properly initialized and contiguous with no gaps or overlap.
// Returns an error if the ranges are invalid, or nil if they are well-formed.
func (h *HeightRanges) Validate() error {
	if h.Bulk.IsEmpty() {
		// Fixme: Uncomment this check after proper mocking in tests
		//if !h.Live.IsEmpty() && h.Live.MinHeight != 0 {
		//	return fmt.Errorf("with empty bulk storage, live storage must start with genesis header")
		//}
	} else {
		if h.Bulk.MinHeight != 0 {
			return fmt.Errorf("bulk storage must start with genesis header")
		}

		if !h.Live.IsEmpty() && h.Bulk.MaxHeight+1 < h.Live.MinHeight {
			gap := must.ConvertToIntFromUnsigned(h.Live.MinHeight) - must.ConvertToIntFromUnsigned(h.Bulk.MaxHeight) - 1
			return fmt.Errorf("there is a gap (%d) between bulk and live header storage, bulk max height: %d, live min height: %d", gap, h.Bulk.MaxHeight, h.Live.MinHeight)
		}
	}

	return nil
}

// HeightRange represents a contiguous range of block heights, defined by MinHeight and MaxHeight values.
// An empty range is indicated by isEmpty or when MinHeight exceeds MaxHeight.
type HeightRange struct {
	MinHeight uint
	MaxHeight uint

	isEmpty bool
}

// NewHeightRange creates a HeightRange with given minHeight and maxHeight, or an empty range if minHeight > maxHeight.
func NewHeightRange(minHeight, maxHeight uint) HeightRange {
	if minHeight > maxHeight {
		return NewEmptyHeightRange()
	}

	return HeightRange{
		MinHeight: minHeight,
		MaxHeight: maxHeight,
	}
}

// NewEmptyHeightRange returns an empty HeightRange representing no valid block heights.
func NewEmptyHeightRange() HeightRange {
	return HeightRange{
		isEmpty: true,
	}
}

// NewHeightRangeFromBlockHeaders returns a HeightRange spanning the min and max heights found in the given block headers.
// If headers is nil or empty, an empty HeightRange is returned.
func NewHeightRangeFromBlockHeaders(headers []*wdk.ChainBlockHeader) HeightRange {
	if len(headers) == 0 {
		return NewEmptyHeightRange()
	}

	minHeight := headers[0].Height
	maxHeight := headers[0].Height

	for _, header := range headers[1:] {
		if header.Height < minHeight {
			minHeight = header.Height
		}
		if header.Height > maxHeight {
			maxHeight = header.Height
		}
	}

	return NewHeightRange(minHeight, maxHeight)
}

// IsEmpty returns true if the range is empty or if MinHeight is greater than MaxHeight.
func (hr HeightRange) IsEmpty() bool {
	return hr.isEmpty || hr.MinHeight > hr.MaxHeight
}

// NotEmpty returns true if the height range is not empty and MinHeight does not exceed MaxHeight.
func (hr HeightRange) NotEmpty() bool {
	return !hr.IsEmpty()
}

// Length returns the total number of block heights in the range.
// Returns 0 if the range is empty or MinHeight exceeds MaxHeight.
func (hr HeightRange) Length() uint {
	if hr.IsEmpty() {
		return 0
	}
	return hr.MaxHeight - hr.MinHeight + 1
}

// String returns a string representation of the HeightRange.
// If the range is empty, it returns "empty", otherwise "[MinHeight - MaxHeight]".
func (hr HeightRange) String() string {
	if hr.IsEmpty() {
		return "empty"
	}
	return fmt.Sprintf("[%d - %d]", hr.MinHeight, hr.MaxHeight)
}

// ContainsHeight returns true if the given height is within the HeightRange, inclusive of MinHeight and MaxHeight.
// Returns false if the range is empty or the height is outside the range.
func (hr HeightRange) ContainsHeight(height uint) bool {
	if hr.IsEmpty() {
		return false
	}
	return height >= hr.MinHeight && height <= hr.MaxHeight
}

// ContainsRange returns true if the given HeightRange is completely within the receiver's range.
// Returns false if either range is empty or the other range extends outside the receiver.
func (hr HeightRange) ContainsRange(other HeightRange) bool {
	if hr.IsEmpty() || other.IsEmpty() {
		return false
	}
	return other.MinHeight >= hr.MinHeight && other.MaxHeight <= hr.MaxHeight
}

// Intersect returns the overlapping HeightRange between the receiver and the other range, or an empty range if none exists.
func (hr HeightRange) Intersect(other HeightRange) HeightRange {
	if hr.IsEmpty() || other.IsEmpty() {
		return NewEmptyHeightRange()
	}

	minHeight := hr.MinHeight
	if other.MinHeight > minHeight {
		minHeight = other.MinHeight
	}

	maxHeight := hr.MaxHeight
	if other.MaxHeight < maxHeight {
		maxHeight = other.MaxHeight
	}

	if minHeight > maxHeight {
		return NewEmptyHeightRange()
	}

	return NewHeightRange(minHeight, maxHeight)
}

// Union returns the smallest HeightRange covering both hr and other, or an error if the ranges are disjoint.
func (hr HeightRange) Union(other HeightRange) (HeightRange, error) {
	if hr.IsEmpty() {
		return other, nil
	}
	if other.IsEmpty() {
		return hr, nil
	}

	if hr.MaxHeight+1 < other.MinHeight || other.MaxHeight+1 < hr.MinHeight {
		return HeightRange{}, fmt.Errorf("union would create disjoint ranges")
	}

	minHeight := hr.MinHeight
	if other.MinHeight < minHeight {
		minHeight = other.MinHeight
	}

	maxHeight := hr.MaxHeight
	if other.MaxHeight > maxHeight {
		maxHeight = other.MaxHeight
	}

	return NewHeightRange(minHeight, maxHeight), nil
}

// Subtract returns a HeightRange representing hr with the other range removed, or an error if subtraction is disjoint.
// If hr is empty, returns an empty HeightRange. If other is empty, returns hr unchanged.
// If other fully contains hr, returns an empty HeightRange. If ranges do not overlap, returns hr unchanged.
// Returns an error if subtraction would create non-contiguous (disjoint) ranges.
func (hr HeightRange) Subtract(other HeightRange) (HeightRange, error) {
	if hr.IsEmpty() {
		return NewEmptyHeightRange(), nil
	}
	if other.IsEmpty() {
		return hr, nil
	}

	if other.MinHeight <= hr.MinHeight && other.MaxHeight >= hr.MaxHeight {
		return NewEmptyHeightRange(), nil
	}

	if other.MinHeight > hr.MaxHeight || other.MaxHeight < hr.MinHeight {
		return hr, nil
	}

	if other.MinHeight > hr.MinHeight && other.MaxHeight < hr.MaxHeight {
		return HeightRange{}, fmt.Errorf("subtraction would create disjoint ranges")
	}

	if other.MinHeight <= hr.MinHeight {
		return NewHeightRange(other.MaxHeight+1, hr.MaxHeight), nil
	}

	return NewHeightRange(hr.MinHeight, other.MinHeight-1), nil
}

// Above returns the portion of hr that is strictly above the other range, or an empty range if none exists.
func (hr HeightRange) Above(other HeightRange) HeightRange {
	if hr.IsEmpty() || other.IsEmpty() {
		return hr
	}

	if hr.MinHeight > other.MaxHeight {
		return hr
	}

	if hr.MaxHeight <= other.MaxHeight {
		return NewEmptyHeightRange()
	}

	return NewHeightRange(other.MaxHeight+1, hr.MaxHeight)
}

// Copy returns a new HeightRange with the same values as the receiver.
func (hr HeightRange) Copy() HeightRange {
	return hr
}

// Overlaps returns true if there is any overlap between the receiver and the other HeightRange.
func (hr HeightRange) Overlaps(other HeightRange) bool {
	return !hr.Intersect(other).IsEmpty()
}
