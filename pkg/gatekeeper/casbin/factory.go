package casbin

import (
	"context"
	"errors"
	"slices"

	"github.com/malekradhouane/magic/internal/core"
	"github.com/malekradhouane/magic/pkg/assertor"
)

type NewCasbinParams struct {
	Logger   core.LoggerContract
	auth     AuthorizationContract
	Enforcer EnforcerDependency
}

type CasbinAuth struct {
	logger        core.LoggerContract
	authorization AuthorizationContract
	closers       []core.Closer
}

var _ CasbinAuthorization = (*CasbinAuth)(nil)

func NewCasbinAuthorization(params NewCasbinParams) (*CasbinAuth, error) {
	auth := &CasbinAuth{
		logger:        params.Logger,
		authorization: params.auth,
		closers:       nil,
	}

	v := assertor.New()
	v.Assert(auth.logger != nil, "logger is missing")
	if err := v.Validate(); err != nil {
		return nil, err
	}

	if auth.authorization == nil {
		params := NewCasbinParams{
			Logger:   auth.logger.With("component", "casbin app"),
			auth:     auth.authorization,
			Enforcer: params.Enforcer,
		}

		app, err := NewAuthorization(params.Enforcer)
		if err != nil {
			auth.logger.Error(err.Error())
			return nil, err
		}

		auth.authorization = app

		auth.closers = append(auth.closers, auth.authorization)
	}

	// Final validation

	v.Assert(auth.authorization != nil, "casbin authorization is missing")
	if err := v.Validate(); err != nil {
		auth.logger.Error(err.Error())
		return nil, err
	}

	return auth, nil
}

func (c *CasbinAuth) Authorization() AuthorizationContract { return c.authorization }

func (c *CasbinAuth) Close(ctx context.Context) error {
	var err error

	for _, closer := range slices.Backward(c.closers) {
		err = errors.Join(err, closer.Close(ctx))
	}
	c.closers = nil

	return err
}
