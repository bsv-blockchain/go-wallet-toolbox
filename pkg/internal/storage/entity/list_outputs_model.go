package entity

// ListOutputsFilter is the filter used to fetch outputs from repo
type ListOutputsFilter struct {
	Basket      string
	Limit       int
	Offset      int
	UserID      int
	IncludeTXID bool
}
