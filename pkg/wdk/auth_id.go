package wdk

// AuthID represents the identity of the user making the request
type AuthID struct {
	IdentityKey string `json:"identityKey"`
	UserID      *int   `json:"userId,omitempty"`
	IsActive    *bool  `json:"isActive,omitempty"`
}
