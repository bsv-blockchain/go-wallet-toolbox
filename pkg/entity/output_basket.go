package entity

import "time"

// OutputBasket represents a user's basket for holding outputs.
type OutputBasket struct {
	Name   string
	UserID int

	CreatedAt time.Time
	UpdatedAt time.Time

	NumberOfDesiredUTXOs    int64
	MinimumDesiredUTXOValue uint64
}

// OutputBasketReadSpecification is used to read OutputBasket entities from the database.
type OutputBasketReadSpecification struct {
	UserID                  *Comparable[int]
	Name                    *Comparable[string]
	NumberOfDesiredUTXOs    *Comparable[int64]
	MinimumDesiredUTXOValue *Comparable[uint64]
}

// OutputBasketUpdateSpecification is used to update OutputBasket entities in the database.
type OutputBasketUpdateSpecification struct {
	UserID                  int
	Name                    *string
	NumberOfDesiredUTXOs    *int64
	MinimumDesiredUTXOValue *uint64
}
