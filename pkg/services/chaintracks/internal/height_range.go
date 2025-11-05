package internal

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type HeightRange struct {
	MinHeight uint
	MaxHeight uint

	isEmpty bool
}

func NewHeightRange(minHeight, maxHeight uint) HeightRange {
	if minHeight > maxHeight {
		return NewEmptyHeightRange()
	}

	return HeightRange{
		MinHeight: minHeight,
		MaxHeight: maxHeight,
	}
}

func NewEmptyHeightRange() HeightRange {
	return HeightRange{
		isEmpty: true,
	}
}

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

func (hr HeightRange) IsEmpty() bool {
	return hr.isEmpty || hr.MinHeight > hr.MaxHeight
}

func (hr HeightRange) Length() uint {
	if hr.IsEmpty() {
		return 0
	}
	return hr.MaxHeight - hr.MinHeight + 1
}

func (hr HeightRange) String() string {
	if hr.IsEmpty() {
		return "empty"
	}
	return fmt.Sprintf("[%d - %d]", hr.MinHeight, hr.MaxHeight)
}

func (hr HeightRange) ContainsHeight(height uint) bool {
	if hr.IsEmpty() {
		return false
	}
	return height >= hr.MinHeight && height <= hr.MaxHeight
}

func (hr HeightRange) ContainsRange(other HeightRange) bool {
	if hr.IsEmpty() || other.IsEmpty() {
		return false
	}
	return other.MinHeight >= hr.MinHeight && other.MaxHeight <= hr.MaxHeight
}

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

func (hr HeightRange) Copy() HeightRange {
	return hr
}
