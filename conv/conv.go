package conv

import (
	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/pkg/interfaces"
)

// ToStoreUser from signup request to store user
func ToStoreUser(req *api.SignUpRequest, hashedPassword string) *interfaces.User {

	return &interfaces.User{
		Email:        req.Email,
		PasswordHash: hashedPassword,
		Username:     req.Username,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		AvatarURL:    req.AvatarURL,
		Provider:     req.Provider,
		ProviderID:   &req.ProviderID,
	}
}
