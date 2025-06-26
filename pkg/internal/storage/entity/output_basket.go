package entity

import "time"

type OutputBasket struct {
	Name   string
	UserID int

	CreatedAt time.Time
	UpdatedAt time.Time

	NumberOfDesiredUTXOs    int64
	MinimumDesiredUTXOValue uint64
}
