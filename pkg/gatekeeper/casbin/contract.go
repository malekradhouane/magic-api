package casbin

import (
	"context"

	"github.com/malekradhouane/magic/internal/core"
)

type CasbinAuthorization interface {
	Authorization() AuthorizationContract
	core.Closer
}

type AuthorizationContract interface {
	GetPermissionsForUser(user string, domain ...string) ([][]string, error)
	Enforce(subject string, obj string, act string) (bool, error)

	Close(ctx context.Context) error
}
