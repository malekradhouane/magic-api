package interfaces

import "time"

// Identity holds authenticated user information data
type Identity struct {
	ID               string `json:"id"`
	UserName         string `json:"userName"`  // user identifier
	FirstName        string `json:"firstName"` // first name
	LastName         string `json:"lastName"`  // last name
	Email            string `json:"email"`
	EmailVerified    bool   `json:"emailVerified"`
	ProfileCompleted bool   `json:"profileCompleted"`
	Role             string `json:"role"`           // user role
	OrganizationID   string `json:"organizationID"` // org identifier (multi-tenancy)
}

// Credential holds authentication info
type Credential struct {
	Username string `json:"username" example:"042"`
	Password string `json:"password" example:"1234"`
}

// Token holds token data
type Token struct {
	AccessToken  string    `json:"accessToken"`           // token
	RefreshToken string    `json:"refreshToken"`          // refresh token to be used for renewal
	Expiration   time.Time `json:"expiration" example:""` // expiration date
}
