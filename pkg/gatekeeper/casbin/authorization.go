package casbin

import (
	"context"

	"github.com/malekradhouane/magic/pkg/assertor"
)

type Authorization struct {
	Enforcer EnforcerDependency
}

var _ AuthorizationContract = (*Authorization)(nil)

func (a *Authorization) Close(_ context.Context) error { return nil }

// create NewAuthorization
func NewAuthorization(enforcer EnforcerDependency) (*Authorization, error) {
	auth := &Authorization{
		Enforcer: enforcer,
	}
	v := assertor.New()
	v.Assert(auth.Enforcer != nil, "enforcer is missing")
	if err := v.Validate(); err != nil {
		return nil, err
	}
	return auth, nil
}

func (a *Authorization) DeletePermissionForUser(subject, object, action string) (bool, error) {
	return a.Enforcer.DeletePermissionForUser(subject, object, action)
}

func (a *Authorization) AddPermissionForUser(subject, object, action string) (bool, error) {
	return a.Enforcer.AddPermissionForUser(subject, object, action)
}

func (a *Authorization) GetPermissionsForUser(user string, domain ...string) ([][]string, error) {
	return a.Enforcer.GetPermissionsForUser(user, domain...)
}

func (a *Authorization) Enforce(subject string, object string, action string) (bool, error) {
	return a.Enforcer.Enforce(subject, object, action)
}
