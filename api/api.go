package api

import (
	"github.com/malekradhouane/magic/pkg/interfaces"
)

// SignUpRequest signup request
type SignUpRequest struct {
	Email          string `json:"email,omitempty" valid:"email,required"`
	Password       string `json:"password,omitempty" valid:"required,minstringlength(8)"`
	Username       string `json:"username"`
	FirstName      string `json:"firstName,omitempty"`
	LastName       string `json:"lastName,omitempty"`
	AvatarURL      string `json:"avatarURL,omitempty"`
	Role           string `json:"-"`
	Location       string `json:"location,omitempty"`
	Provider       string `json:"provider,omitempty"`
	ProviderID     string `json:"providerID,omitempty"`
	PhoneNumber    string `json:"phoneNumber,omitempty"`
	DateOfBirth    string `json:"dateOfBirth,omitempty"`
	Gender         string `json:"gender,omitempty"`
	OrganizationID string `json:"organizationID,omitempty"`
}

// Login represents auth data
type Login struct {
	Email    string `json:"email" valid:"email~Invalid email"`
	Password string `json:"password" valid:"required~The password is required"`
}

// AuthenticatedUser represents an authed user
type AuthenticatedUser struct {
	ID    string `json:"id,omitempty"`
	Email string `json:"email,omitempty"`
}

type ResetPassword struct {
	CurrentEmail string `json:"currentEmail,omitempty" valid:"email,required"`
}

// UpdateUserSelfRequest is the payload a user may submit to update their own
// profile. It deliberately omits any field that could be used to escalate
// privileges (role, is_superuser, is_active, email_verified, …).
type UpdateUserSelfRequest struct {
	Username    string  `json:"username"`
	FirstName   string  `json:"first_name"`
	LastName    string  `json:"last_name"`
	AvatarURL   string  `json:"avatar_url"`
	PhoneNumber string  `json:"phone_number"`
	DateOfBirth *string `json:"date_of_birth"`
	Gender      string  `json:"gender"`
	Locale      string  `json:"locale"`
}

// ToUser converts the safe self-update payload to a domain user. Privileged
// fields are intentionally left at their zero value so they cannot be
// mass-assigned via a partial update.
func (u *UpdateUserSelfRequest) ToUser() *interfaces.User {
	return &interfaces.User{
		Username:    u.Username,
		FirstName:   u.FirstName,
		LastName:    u.LastName,
		AvatarURL:   u.AvatarURL,
		PhoneNumber: u.PhoneNumber,
		DateOfBirth: u.DateOfBirth,
		Gender:      u.Gender,
		Locale:      u.Locale,
	}
}

// UpdateUserRequest is the admin-only payload that may also flip privileged
// flags (is_active, is_superuser, email_verified, …). Never bind this struct
// from a non-admin route.
type UpdateUserRequest struct {
	Username      string  `json:"username"`
	Email         string  `json:"email"`
	FirstName     string  `json:"first_name"`
	LastName      string  `json:"last_name"`
	AvatarURL     string  `json:"avatar_url"`
	PhoneNumber   string  `json:"phone_number"`
	DateOfBirth   *string `json:"date_of_birth"`
	Gender        string  `json:"gender"`
	Locale        string  `json:"locale"`
	EmailVerified bool    `json:"email_verified"`
	PhoneVerified bool    `json:"phone_verified"`
	IsActive      bool    `json:"is_active"`
	IsSuperuser   bool    `json:"is_superuser"`
}

func (u *UpdateUserRequest) ToUser() *interfaces.User {
	return &interfaces.User{
		Username:      u.Username,
		FirstName:     u.FirstName,
		LastName:      u.LastName,
		AvatarURL:     u.AvatarURL,
		PhoneNumber:   u.PhoneNumber,
		DateOfBirth:   u.DateOfBirth,
		Gender:        u.Gender,
		Locale:        u.Locale,
		EmailVerified: u.EmailVerified,
		PhoneVerified: u.PhoneVerified,
		IsActive:      u.IsActive,
		IsSuperuser:   u.IsSuperuser,
	}
}
