package entity

import (
	"time"
)

type Label struct {
	CreatedAt time.Time
	UpdatedAt time.Time

	Name   string
	UserID int
}

type LabelMap struct {
	CreatedAt time.Time
	UpdatedAt time.Time

	Name          string
	UserID        int
	TransactionID uint
}
