package entity

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// User represents a user in the system.
type User struct {
	ID            int
	IdentityKey   string
	ActiveStorage string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// UserReadSpecification defines the criteria for reading users.
type UserReadSpecification struct {
	ID            *int
	IdentityKey   *Comparable[string]
	ActiveStorage *Comparable[string]
}

// UserUpdateSpecification defines the criteria for updating users.
type UserUpdateSpecification struct {
	ID            int
	IdentityKey   *string
	ActiveStorage *string
}

// ToWDK converts the User entity to a WDK TableUser.
func (u *User) ToWDK() *wdk.TableUser {
	return &wdk.TableUser{
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
		UserID:        u.ID,
		IdentityKey:   u.IdentityKey,
		ActiveStorage: u.ActiveStorage,
	}
}
