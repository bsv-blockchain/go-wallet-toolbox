package entity

import "time"

type UserUTXO struct {
	UserID             int
	OutputID           uint
	BasketName         string
	Satoshis           uint64
	EstimatedInputSize uint64
	CreatedAt          time.Time
	ReservedByID       *uint
}
