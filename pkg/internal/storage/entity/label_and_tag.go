package entity

import (
	"time"
)

type TxLabel struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time

	Label     string
	UserID    int
	IsDeleted bool
}

type TxLabelsMap struct {
	CreatedAt time.Time
	UpdatedAt time.Time

	TxLabelID     uint
	TransactionID uint
	IsDeleted     bool
}

type OutputTag struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time

	Tag       string
	UserID    int
	IsDeleted bool
}

type OutputTagsMap struct {
	CreatedAt time.Time
	UpdatedAt time.Time

	OutputTagID uint
	OutputID    uint
	IsDeleted   bool
}
