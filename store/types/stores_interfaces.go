package types

import (
	"context"

	"github.com/malekradhouane/magic/pkg/interfaces"
)

// Higher level application code should not have to rely on these interfaces,
// and rather expose its method required by exposing its own data store usage interface.

// DataStoreInterface list the minimum core features of a data store
type DataStoreInterface interface {
	Ping() error
}

// UserStore represents the interfaces to manage users storage
type UserStore interface {
	CreateUser(ctx context.Context, user *interfaces.User, companyID string, role string) (*interfaces.User, error)
	Get(context.Context, string) (*interfaces.User, error)
	GetUserByEmail(context.Context, string) (*interfaces.User, error)
	GetUsers(context.Context) ([]*interfaces.User, error)
	GetUsersByRole(ctx context.Context, role string) ([]*interfaces.User, error)
	IsEmailTaken(context.Context, string) (bool, error)
	Authenticate(context.Context, *interfaces.Credential) (*interfaces.User, error)
	FindByEmailAndProvider(context.Context, string, string) (*interfaces.User, error)
	UpdateUser(ctx context.Context, id string, user *interfaces.User) (*interfaces.User, error)
	UpdateUserFields(ctx context.Context, id string, fields map[string]interface{}) (*interfaces.User, error)
	DeleteUser(ctx context.Context, id string) error
	Close() error

	// Validation token operations
	CreateValidationToken(ctx context.Context, token *interfaces.ValidationToken) error
	GetValidationToken(ctx context.Context, token string) (*interfaces.ValidationToken, error)
	DeleteValidationToken(ctx context.Context, token string) error
}
