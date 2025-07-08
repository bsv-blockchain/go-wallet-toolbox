package entity

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type User struct {
	ID            int
	IdentityKey   string
	ActiveStorage string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (u *User) ToWDK() *wdk.TableUser {
	return &wdk.TableUser{
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
		UserID:        u.ID,
		IdentityKey:   u.IdentityKey,
		ActiveStorage: u.ActiveStorage,
	}
}
