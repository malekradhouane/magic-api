package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/store/types"
	"github.com/malekradhouane/magic/utils/encrypt"
)

var (
	_ types.UserStore = &UserStore{}

	theUserStoreMtx sync.Mutex
	theUserStore    *UserStore
)

type UserStore struct {
	*Client
}

// NewUserStore initializes a UserStore (singleton-style)
func NewUserStore() (*UserStore, error) {
	theUserStoreMtx.Lock()
	defer theUserStoreMtx.Unlock()

	if theUserStore != nil {
		return theUserStore, nil
	}
	MustClientInitialized(client)
	theUserStore = &UserStore{
		Client: client,
	}

	logrus.Info("UserStore created")
	return theUserStore, nil
}

// CreateUser creates a user (generates an ID if missing).
func (us *UserStore) CreateUser(ctx context.Context, user *interfaces.User, companyID, role string) (*interfaces.User, error) {
	if user == nil {
		return nil, errs.ErrUserNil
	}

	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	// Actually insert the user into the database
	if err := us.session.GetDB().Create(user).Error; err != nil {
		if isEmailDuplicateError(wrapPgError(err)) {
			return nil, errs.ErrEmailTaken
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// Get retrieves a user by ID
func (us *UserStore) Get(ctx context.Context, id string) (*interfaces.User, error) {
	u := new(interfaces.User)
	err := us.session.GetDB().Model(&interfaces.User{}).Where("id = ?", id).Take(u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return u, nil
}

// GetUsers lists all users
func (us *UserStore) GetUsers(ctx context.Context) ([]*interfaces.User, error) {
	var users []*interfaces.User
	if err := us.session.GetDB().Find(&users).Error; err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	return users, nil
}

// IsEmailTaken returns whether an email already exists
func (us *UserStore) IsEmailTaken(ctx context.Context, email string) (bool, error) {
	var count int64
	if err := us.session.GetDB().
		Model(&interfaces.User{}).
		Where("email = ?", email).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check email: %w", err)
	}
	return count > 0, nil
}

// GetUserByEmail retrieves a user by email
func (us *UserStore) GetUserByEmail(ctx context.Context, email string) (*interfaces.User, error) {
	u := new(interfaces.User)
	err := us.session.GetDB().Model(&interfaces.User{}).Where("email = ?", email).Take(u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return u, nil
}

// FindByEmailAndProvider retrieves a user by email and provider
func (us *UserStore) FindByEmailAndProvider(ctx context.Context, email string, provider string) (*interfaces.User, error) {
	u := new(interfaces.User)
	err := us.session.GetDB().
		Model(&interfaces.User{}).
		Where("email = ? AND provider = ?", email, provider).
		Take(u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to get user by email/provider: %w", err)
	}
	return u, nil
}

// Authenticate verifies credentials and returns the user
func (us *UserStore) Authenticate(ctx context.Context, login *interfaces.Credential) (*interfaces.User, error) {
	user := new(interfaces.User)
	err := us.session.GetDB().Model(&interfaces.User{}).Where("username = ?", login.Username).Take(user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to authenticate user: %w", err)
	}
	if err := encrypt.VerifyPassword(user.PasswordHash, login.Password); err != nil {
		return nil, err
	}
	return user, nil
}

// UpdateUser updates non-zero fields of a user
func (us *UserStore) UpdateUser(ctx context.Context, id string, user *interfaces.User) (*interfaces.User, error) {
	if user == nil {
		return nil, errs.ErrUserNil
	}
	if id == "" {
		return nil, errs.ErrUserIDMissing
	}

	err := withTransaction(us.session.GetDB(), func(tx *gorm.DB) error {
		// Ensure user exists
		existing := new(interfaces.User)
		if err := tx.Where("id = ?", id).Take(existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errs.ErrNoSuchEntity
			}
			return fmt.Errorf("failed to get user: %w", err)
		}

		// Update fields
		if err := tx.Model(existing).Updates(user).Error; err != nil {
			return fmt.Errorf("failed to update user: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Reload
	updated := new(interfaces.User)
	if err := us.session.GetDB().Where("id = ?", id).Take(updated).Error; err != nil {
		return nil, fmt.Errorf("failed to reload user: %w", err)
	}
	return updated, nil
}

// UpdateUserFields updates explicit fields using a map
func (us *UserStore) UpdateUserFields(ctx context.Context, id string, fields map[string]interface{}) (*interfaces.User, error) {
	if id == "" {
		return nil, errs.ErrUserIDMissing
	}
	if len(fields) == 0 {
		return nil, errs.ErrEmptyUpdate
	}

	db := us.session.GetDB()
	if err := db.Model(&interfaces.User{}).Where("id = ?", id).Take(&interfaces.User{}).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	if err := db.Model(&interfaces.User{}).Where("id = ?", id).Updates(fields).Error; err != nil {
		return nil, fmt.Errorf("failed to update user fields: %w", err)
	}

	out := new(interfaces.User)
	if err := db.Where("id = ?", id).Take(out).Error; err != nil {
		return nil, fmt.Errorf("failed to reload user: %w", err)
	}
	return out, nil
}

// DeleteUser deletes a user by ID (cascades on user_companies)
func (us *UserStore) DeleteUser(ctx context.Context, id string) error {
	if id == "" {
		return errs.ErrUserIDMissing
	}

	return withTransaction(us.session.GetDB(), func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&interfaces.User{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return fmt.Errorf("failed to check user existence: %w", err)
		}
		if count == 0 {
			return errs.ErrNoSuchEntity
		}

		if err := tx.Where("id = ?", id).Delete(&interfaces.User{}).Error; err != nil {
			return fmt.Errorf("failed to delete user: %w", err)
		}
		return nil
	})
}

// CreateValidationToken creates a new validation token
func (us *UserStore) CreateValidationToken(ctx context.Context, token *interfaces.ValidationToken) error {
	if token == nil {
		return fmt.Errorf("validation token is nil")
	}

	db := us.session.GetDB()
	if err := db.Create(token).Error; err != nil {
		return fmt.Errorf("failed to create validation token: %w", err)
	}
	return nil
}

// GetValidationToken retrieves a validation token by its value
func (us *UserStore) GetValidationToken(ctx context.Context, tokenValue string) (*interfaces.ValidationToken, error) {
	if tokenValue == "" {
		return nil, fmt.Errorf("token value is required")
	}

	var token interfaces.ValidationToken
	db := us.session.GetDB()
	if err := db.Where("token = ?", tokenValue).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to get validation token: %w", err)
	}
	return &token, nil
}

// DeleteValidationToken deletes a validation token
func (us *UserStore) DeleteValidationToken(ctx context.Context, tokenValue string) error {
	if tokenValue == "" {
		return fmt.Errorf("token value is required")
	}

	db := us.session.GetDB()
	if err := db.Where("token = ?", tokenValue).Delete(&interfaces.ValidationToken{}).Error; err != nil {
		return fmt.Errorf("failed to delete validation token: %w", err)
	}
	return nil
}
