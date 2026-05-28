package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/conv"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/store/types"
	"github.com/malekradhouane/magic/utils/encrypt"
	"github.com/malekradhouane/magic/utils/phone"
)

// UserService user service
type UserService struct {
	userStore types.UserStore
	logger    *logrus.Logger
}

// NewUserService constructs a new UserService
func NewUserService(us types.UserStore, logger *logrus.Logger) *UserService {
	if logger == nil {
		logger = logrus.New()
	}
	return &UserService{
		userStore: us,
		logger:    logger,
	}
}

func (us *UserService) Create(ctx context.Context, req *api.SignUpRequest) (*api.AuthenticatedUser, error) {
	email := strings.ToLower(req.Email)
	us.logger.WithField("email", email).Info("Attempting to create user")

	taken, err := us.userStore.IsEmailTaken(ctx, email)
	if err != nil {
		us.logger.WithError(err).WithField("email", email).Error("Failed to check if email is taken")
		return nil, err
	}
	if taken {
		us.logger.WithField("email", email).Warn("Email already taken")
		return nil, errs.ErrEmailTaken
	}

	hashedPassword, err := encrypt.Hash(req.Password)
	if err != nil {
		us.logger.WithError(err).WithField("email", email).Error("Failed to hash password")
		return nil, err
	}
	req.Password = string(hashedPassword)

	user, err := us.userStore.CreateUser(ctx, conv.ToStoreUser(req, string(hashedPassword)), "", "user")
	if err != nil {
		us.logger.WithError(err).WithField("email", email).Error("Failed to create user in store")
		return nil, err
	}

	us.logger.WithFields(logrus.Fields{
		"email": user.Email,
		"id":    user.ID,
	}).Info("User created successfully")

	return &api.AuthenticatedUser{
		ID:    fmt.Sprint(user.ID),
		Email: user.Email,
	}, nil
}

func (us *UserService) CreateWithPassword(ctx context.Context, req *api.SignUpRequest) (*api.AuthenticatedUser, error) {
	email := strings.ToLower(req.Email)
	us.logger.WithField("email", email).Info("Attempting to create user with password")

	taken, err := us.userStore.IsEmailTaken(ctx, email)
	if err != nil {
		us.logger.WithError(err).WithField("email", email).Error("Failed to check if email is taken")
		return nil, err
	}
	if taken {
		us.logger.WithField("email", email).Warn("Email already taken")
		return nil, errs.ErrEmailTaken
	}

	hashedPassword, err := encrypt.Hash(req.Password)
	if err != nil {
		us.logger.WithError(err).WithField("email", email).Error("Failed to hash password")
		return nil, err
	}

	user := &interfaces.User{
		CreatedAt:    time.Time{},
		Email:        email,
		PasswordHash: string(hashedPassword),
		Provider:     "manual",
		Role:         "user",
		Username:     req.Username,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		AvatarURL:    req.AvatarURL,
		PhoneNumber:  req.PhoneNumber,
		DateOfBirth:  &req.DateOfBirth,
		Gender:       req.Gender,
	}

	// If you need to associate with a company, pass companyID and role instead of empty strings.
	newUser, err := us.userStore.CreateUser(ctx, user, "", req.Role)
	if err != nil {
		us.logger.WithError(err).WithField("email", email).Error("Failed to create user in store")
		return nil, err
	}
	us.logger.WithFields(logrus.Fields{"id": newUser.ID, "email": newUser.Email}).Info("User created successfully")

	// fmt.Sprint supports both UUID (Stringer) and numeric IDs
	return &api.AuthenticatedUser{
		ID:    fmt.Sprint(newUser.ID),
		Email: newUser.Email,
	}, nil
}

func (us *UserService) CreateOrGetOAuthUser(ctx context.Context, payload *api.SignUpRequest) (*interfaces.User, error) {
	email := strings.ToLower(payload.Email)
	user, err := us.userStore.FindByEmailAndProvider(ctx, email, payload.Provider)
	if err == nil && user != nil {
		return user, nil
	}

	// If user doesn't exist, create
	user, err = us.userStore.CreateUser(ctx, &interfaces.User{
		Email:      email,
		FirstName:  payload.FirstName,
		LastName:   payload.LastName,
		AvatarURL:  payload.AvatarURL,
		Provider:   payload.Provider,
		ProviderID: &payload.ProviderID,
	}, "", "")
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (us *UserService) GetUser(ctx context.Context, id string) (*interfaces.User, error) {
	us.logger.WithField("id", id).Debug("Fetching user")

	user, err := us.userStore.Get(ctx, id)
	if err != nil {
		if errs.IsNoSuchEntityError(err) {
			us.logger.WithField("id", id).Warn("User not found")
			return nil, errs.ErrNoSuchEntity
		}
		us.logger.WithError(err).WithField("id", id).Error("Failed to fetch user")
		return nil, err
	}

	us.logger.WithFields(logrus.Fields{
		"id":    user.ID,
		"email": user.Email,
	}).Debug("Fetched user successfully")

	return user, nil
}

func (us *UserService) GetUsers(ctx context.Context) ([]*interfaces.User, error) {
	us.logger.Debug("Fetching users")
	users, err := us.userStore.GetUsers(ctx)
	if err != nil {
		us.logger.WithError(err).Error("Failed to fetch users")
		return nil, err
	}
	us.logger.WithField("count", len(users)).Debug("Fetched users successfully")
	return users, nil
}

func (us *UserService) DeleteUser(ctx context.Context, id string) error {
	us.logger.WithField("id", id).Info("Attempting to delete user")

	err := us.userStore.DeleteUser(ctx, id)
	if err != nil {
		if errs.IsNoSuchEntityError(err) {
			us.logger.WithField("id", id).Warn("User not found for deletion")
			return errs.ErrNoSuchEntity
		}
		us.logger.WithError(err).WithField("id", id).Error("Failed to delete user")
		return fmt.Errorf("failed to delete user: %v", err)
	}

	us.logger.WithField("id", id).Info("User deleted successfully")
	return nil
}

// UpdateUser updates a user's information
func (us *UserService) UpdateUser(ctx context.Context, id string, req *api.UpdateUserRequest) (*interfaces.User, error) {
	// Validate & normalize the phone number when the caller supplies one.
	// The order workflow requires a valid Tunisian number so the admin can
	// call the customer to confirm orders by phone.
	if strings.TrimSpace(req.PhoneNumber) != "" {
		normalized, err := phone.Normalize(req.PhoneNumber)
		if err != nil {
			return nil, err
		}
		req.PhoneNumber = normalized
	}

	updatedUser, err := us.userStore.UpdateUser(ctx, id, req.ToUser())
	if err != nil {
		if errs.IsNoSuchEntityError(err) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to update user: %v", err)
	}

	return updatedUser, nil
}

// UpdateUserFields updates specific fields of a user
func (us *UserService) UpdateUserFields(ctx context.Context, id string, fields map[string]interface{}) (*interfaces.User, error) {
	if raw, ok := fields["phone_number"]; ok {
		if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
			normalized, err := phone.Normalize(s)
			if err != nil {
				return nil, err
			}
			fields["phone_number"] = normalized
		}
	}

	updatedUser, err := us.userStore.UpdateUserFields(ctx, id, fields)
	if err != nil {
		if errs.IsNoSuchEntityError(err) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to update user fields: %v", err)
	}

	return updatedUser, nil
}
