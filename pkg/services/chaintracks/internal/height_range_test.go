package internal

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeightRange_IsEmpty(t *testing.T) {
	tests := map[string]struct {
		hr     HeightRange
		expect bool
	}{
		"isEmpty true": {NewEmptyHeightRange(), true},
		"min>max":      {NewHeightRange(5, 4), true},
		"min=max":      {NewHeightRange(4, 4), false},
		"normal":       {NewHeightRange(1, 10), false},
		"zero range":   {NewHeightRange(0, 0), false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expect, tc.hr.IsEmpty())
		})
	}
}

func TestHeightRange_Length(t *testing.T) {
	tests := map[string]struct {
		hr     HeightRange
		expect uint
	}{
		"empty by isEmpty":   {NewEmptyHeightRange(), 0},
		"empty by min > max": {NewHeightRange(5, 4), 0},
		"min=max (one)":      {NewHeightRange(3, 3), 1},
		"normal range":       {NewHeightRange(1, 5), 5},
		"zero range":         {NewHeightRange(0, 0), 1},
		"max < min":          {NewHeightRange(10, 5), 0},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expect, tc.hr.Length())
		})
	}
}

func TestHeightRange_String(t *testing.T) {
	tests := map[string]struct {
		hr     HeightRange
		expect string
	}{
		"empty by isEmpty":   {NewEmptyHeightRange(), "empty"},
		"empty by min > max": {NewHeightRange(10, 2), "empty"},
		"zero range":         {NewHeightRange(0, 0), "[0 - 0]"},
		"min=max":            {NewHeightRange(5, 5), "[5 - 5]"},
		"normal range":       {NewHeightRange(2, 8), "[2 - 8]"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expect, tc.hr.String())
		})
	}
}

func TestHeightRange_ContainsHeight(t *testing.T) {
	tests := map[string]struct {
		hr     HeightRange
		height uint
		expect bool
	}{
		"empty by isEmpty": {NewEmptyHeightRange(), 5, false},
		"empty by min>max": {NewHeightRange(3, 2), 3, false},
		"in range":         {NewHeightRange(2, 5), 3, true},
		"at min":           {NewHeightRange(2, 5), 2, true},
		"at max":           {NewHeightRange(2, 5), 5, true},
		"below min":        {NewHeightRange(2, 5), 0, false},
		"above max":        {NewHeightRange(2, 5), 10, false},
		"exact zero":       {NewHeightRange(0, 0), 0, true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expect, tc.hr.ContainsHeight(tc.height))
		})
	}
}

func TestHeightRange_ContainsRange(t *testing.T) {
	tests := map[string]struct {
		hr, other HeightRange
		expect    bool
	}{
		"both empty":            {NewEmptyHeightRange(), NewEmptyHeightRange(), false},
		"self empty":            {NewEmptyHeightRange(), NewHeightRange(1, 2), false},
		"other empty":           {NewHeightRange(0, 9), NewEmptyHeightRange(), false},
		"other strictly inside": {NewHeightRange(2, 10), NewHeightRange(4, 9), true},
		"same":                  {NewHeightRange(2, 10), NewHeightRange(2, 10), true},
		"other equals min":      {NewHeightRange(2, 10), NewHeightRange(2, 6), true},
		"other equals max":      {NewHeightRange(2, 10), NewHeightRange(4, 10), true},
		"outside below":         {NewHeightRange(4, 8), NewHeightRange(1, 5), false},
		"outside above":         {NewHeightRange(4, 8), NewHeightRange(7, 10), false},
		"overlapping left":      {NewHeightRange(4, 8), NewHeightRange(2, 6), false},
		"overlapping right":     {NewHeightRange(4, 8), NewHeightRange(6, 10), false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expect, tc.hr.ContainsRange(tc.other))
		})
	}
}

func TestHeightRange_Intersect(t *testing.T) {
	tests := map[string]struct {
		hr, other HeightRange
		expect    HeightRange
	}{
		"both empty":            {NewEmptyHeightRange(), NewEmptyHeightRange(), NewEmptyHeightRange()},
		"self empty":            {NewEmptyHeightRange(), NewHeightRange(1, 2), NewEmptyHeightRange()},
		"other empty":           {NewHeightRange(0, 9), NewEmptyHeightRange(), NewEmptyHeightRange()},
		"no overlap":            {NewHeightRange(5, 8), NewHeightRange(10, 12), NewEmptyHeightRange()},
		"overlap":               {NewHeightRange(2, 8), NewHeightRange(6, 10), NewHeightRange(6, 8)},
		"covers self":           {NewHeightRange(4, 8), NewHeightRange(2, 10), NewHeightRange(4, 8)},
		"covers other":          {NewHeightRange(2, 10), NewHeightRange(4, 8), NewHeightRange(4, 8)},
		"same":                  {NewHeightRange(2, 10), NewHeightRange(2, 10), NewHeightRange(2, 10)},
		"zero overlap endpoint": {NewHeightRange(2, 5), NewHeightRange(5, 6), NewHeightRange(5, 5)},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.hr.Intersect(tc.other)
			assert.Equal(t, tc.expect, got)
		})
	}
}

func TestHeightRange_Union_Success(t *testing.T) {
	tests := map[string]struct {
		hr, other HeightRange
		expect    HeightRange
	}{
		"self empty":  {NewEmptyHeightRange(), NewHeightRange(1, 5), NewHeightRange(1, 5)},
		"other empty": {NewHeightRange(2, 8), NewEmptyHeightRange(), NewHeightRange(2, 8)},
		"both empty":  {NewEmptyHeightRange(), NewEmptyHeightRange(), NewEmptyHeightRange()},
		"joining":     {NewHeightRange(4, 8), NewHeightRange(2, 6), NewHeightRange(2, 8)},
		"overlap":     {NewHeightRange(4, 8), NewHeightRange(6, 10), NewHeightRange(4, 10)},
		"subset":      {NewHeightRange(4, 8), NewHeightRange(5, 6), NewHeightRange(4, 8)},
		"same":        {NewHeightRange(2, 3), NewHeightRange(2, 3), NewHeightRange(2, 3)},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tc.hr.Union(tc.other)
			require.NoError(t, err)
			require.Equal(t, tc.expect, got)
		})
	}
}

func TestHeightRange_Union_Error(t *testing.T) {
	hr := NewHeightRange(9, 10)
	other := NewHeightRange(2, 3)
	_, err := hr.Union(other)
	require.Error(t, err)
}

func TestHeightRange_Subtract_Success(t *testing.T) {
	tests := map[string]struct {
		hr, other HeightRange
		expect    HeightRange
	}{
		"empty self":          {NewEmptyHeightRange(), NewHeightRange(2, 5), NewEmptyHeightRange()},
		"empty other":         {NewHeightRange(2, 5), NewEmptyHeightRange(), NewHeightRange(2, 5)},
		"other covers self":   {NewHeightRange(2, 5), NewHeightRange(1, 10), NewEmptyHeightRange()},
		"disjoint left":       {NewHeightRange(10, 20), NewHeightRange(1, 5), NewHeightRange(10, 20)},
		"disjoint right":      {NewHeightRange(10, 20), NewHeightRange(21, 30), NewHeightRange(10, 20)},
		"subtract left edge":  {NewHeightRange(5, 10), NewHeightRange(2, 6), NewHeightRange(7, 10)},
		"subtract right edge": {NewHeightRange(5, 10), NewHeightRange(8, 11), NewHeightRange(5, 7)},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tc.hr.Subtract(tc.other)
			require.NoError(t, err)
			assert.Equal(t, tc.expect, got)
		})
	}
}

func TestHeightRange_Subtract_Error(t *testing.T) {
	hr := NewHeightRange(5, 10)
	other := NewHeightRange(7, 8)
	_, err := hr.Subtract(other)
	require.Error(t, err)
}

func TestHeightRange_Above(t *testing.T) {
	tests := map[string]struct {
		hr, other HeightRange
		expect    HeightRange
	}{
		"self empty":       {NewEmptyHeightRange(), NewHeightRange(1, 3), NewEmptyHeightRange()},
		"other empty":      {NewHeightRange(1, 10), NewEmptyHeightRange(), NewHeightRange(1, 10)},
		"self fully above": {NewHeightRange(21, 30), NewHeightRange(0, 20), NewHeightRange(21, 30)},
		"all below":        {NewHeightRange(5, 10), NewHeightRange(8, 12), NewEmptyHeightRange()},
		"overlap at edge":  {NewHeightRange(5, 10), NewHeightRange(7, 9), NewHeightRange(10, 10)},
		"overlap right":    {NewHeightRange(5, 15), NewHeightRange(10, 11), NewHeightRange(12, 15)},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.hr.Above(tc.other)
			assert.Equal(t, tc.expect, got)
		})
	}
}

func TestHeightRange_Copy(t *testing.T) {
	orig := HeightRange{MinHeight: 2, MaxHeight: 5}
	cp := orig.Copy()
	assert.Equal(t, orig, cp)
	cp.MinHeight = 123
	assert.NotEqual(t, orig, cp)
}

func TestNewHeightRange(t *testing.T) {
	tests := map[string]struct {
		min, max uint
		expect   HeightRange
	}{
		"normal":    {2, 5, NewHeightRange(2, 5)},
		"min = max": {3, 3, NewHeightRange(3, 3)},
		"min > max": {5, 2, NewEmptyHeightRange()},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			hr := NewHeightRange(tc.min, tc.max)
			assert.Equal(t, tc.expect, hr)
		})
	}
}

func TestNewEmptyHeightRange(t *testing.T) {
	e := NewEmptyHeightRange()
	assert.True(t, e.IsEmpty())
	assert.Equal(t, uint(0), e.Length())
}

func TestNewHeightRangeFromBlockHeaders(t *testing.T) {
	tests := map[string]struct {
		headers []*wdk.ChainBlockHeader
		expect  HeightRange
	}{
		"empty":          {nil, NewEmptyHeightRange()},
		"one block":      {[]*wdk.ChainBlockHeader{{Height: 5}}, NewHeightRange(5, 5)},
		"two ascending":  {[]*wdk.ChainBlockHeader{{Height: 2}, {Height: 8}}, NewHeightRange(2, 8)},
		"two descending": {[]*wdk.ChainBlockHeader{{Height: 8}, {Height: 2}}, NewHeightRange(2, 8)},
		"mixed":          {[]*wdk.ChainBlockHeader{{Height: 9}, {Height: 2}, {Height: 7}}, NewHeightRange(2, 9)},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			hr := NewHeightRangeFromBlockHeaders(tc.headers)
			assert.Equal(t, tc.expect, hr)
		})
	}
}

func TestHeightRange_StructZeroValue_IsEmpty(t *testing.T) {
	var hr HeightRange
	assert.False(t, hr.isEmpty)
	assert.False(t, hr.IsEmpty())
	assert.Equal(t, "[0 - 0]", hr.String())
}
