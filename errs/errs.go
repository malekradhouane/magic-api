package errs

import "errors"

var (
	// User-related
	ErrUserNil        = errors.New("user is nil")
	ErrUserIDRequired = errors.New("user ID is required")
	ErrUserIDMissing  = errors.New("user id is required")
	ErrEmailTaken     = errors.New("email address already taken")

	// Company-related
	ErrCompanyNotFound      = errors.New("company not found")
	ErrCompanyAssociation   = errors.New("failed to associate user with company")
	ErrCompanyAlreadyLinked = errors.New("user already linked to company")
	ErrCompanyNil           = errors.New("company is nil")
	ErrCompanyIDRequired    = errors.New("company ID is required")

	// Organization-related
	ErrOrgIDRequired          = errors.New("organization ID is required")
	ErrOrganizationNil        = errors.New("organization is nil")
	ErrOrganizationIDRequired = errors.New("organization ID is required")
	ErrOrganizationNotFound   = errors.New("organization not found")

	// Generic
	ErrNoSuchEntity = errors.New("no such entity")
	ErrEmptyUpdate  = errors.New("no fields to update")

	// Promo-related
	ErrPromoUsageLimitReached = errors.New("promo usage limit reached")
)

// IsNoSuchEntityError checks if error is ErrNoSuchEntity
func IsNoSuchEntityError(e error) bool {
	return errors.Is(e, ErrNoSuchEntity)
}

// IsEmptyUpdateError checks if error is ErrEmptyUpdate
func IsEmptyUpdateError(e error) bool {
	return errors.Is(e, ErrEmptyUpdate)
}
