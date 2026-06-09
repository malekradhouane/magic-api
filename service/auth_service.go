package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/pkg/mailer"
	"github.com/malekradhouane/magic/pkg/mailer/template"
	"github.com/malekradhouane/magic/store/types"
	"github.com/malekradhouane/magic/utils/encrypt"
)

// AuthIdentity represents an authentication method for a user
type AuthIdentity struct {
	ID           string `gorm:"primaryKey"`
	UserID       string `gorm:"not null;index"`
	Provider     string `gorm:"not null"` // "password", "google", "github", etc.
	ProviderID   string `gorm:"index"`    // OAuth provider user ID
	PasswordHash string // Only for password provider
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AuthService handles authentication operations
type AuthService struct {
	userStore     types.UserStore
	logger        *logrus.Logger
	mailer        mailer.Mailer
	mailFromName  string
	mailFromEmail string
}

// NewAuthService creates a new auth service
func NewAuthService(
	userStore types.UserStore,
	logger *logrus.Logger,
	mailer mailer.Mailer,
	mailFromName, mailFromEmail string,
) *AuthService {
	if logger == nil {
		logger = logrus.New()
	}
	if strings.TrimSpace(mailFromName) == "" {
		mailFromName = "Magic"
	}
	if strings.TrimSpace(mailFromEmail) == "" {
		mailFromEmail = "noreply@magic.fr"
	}
	return &AuthService{
		userStore:     userStore,
		logger:        logger,
		mailer:        mailer,
		mailFromName:  mailFromName,
		mailFromEmail: mailFromEmail,
	}
}

// SignUpResult contains the result of a signup operation
type SignUpResult struct {
	User      *interfaces.User
	IsNewUser bool
	Token     string
}

// SignUpWithPassword creates a new user or adds password auth to existing user
func (as *AuthService) SignUpWithPassword(ctx context.Context, req *api.SignUpRequest) (*SignUpResult, error) {
	email := strings.ToLower(req.Email)

	// Check if user exists
	user, err := as.userStore.GetUserByEmail(ctx, email)
	if err != nil && !errs.IsNoSuchEntityError(err) {
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}

	isNewUser := user == nil
	if !isNewUser {
		as.logger.WithField("email", email).Warn("Signup attempted with existing email")
		return nil, errs.ErrEmailTaken
	}

	// Hash password
	hashedPassword, err := encrypt.Hash(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		username = email
	}

	// Default role is "user" for security, but allow override if explicitly set
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "user"
	}

	user = &interfaces.User{
		Email:     email,
		Username:  username,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		AvatarURL: req.AvatarURL,
		Role:      role,
		CreatedAt: time.Now(),
	}

	user, err = as.userStore.CreateUser(ctx, user, "", role)
	if err != nil {
		if errors.Is(err, errs.ErrEmailTaken) {
			return nil, errs.ErrEmailTaken
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// In a real implementation, you would create an AuthIdentity record here
	// For now, we'll update the user's password hash
	user.PasswordHash = string(hashedPassword)
	user.Provider = "password"

	if _, err := as.userStore.UpdateUserFields(ctx, user.ID.String(), map[string]interface{}{
		"password_hash": string(hashedPassword),
		"provider":      "password",
	}); err != nil {
		return nil, fmt.Errorf("failed to save password: %w", err)
	}

	as.logger.WithFields(logrus.Fields{
		"email": email,
		"new":   isNewUser,
	}).Info("User signed up with password")

	// Send activation email for new users
	if isNewUser && !user.EmailVerified {
		token, err := as.generateActivationToken(ctx, user.ID.String())
		if err != nil {
			as.logger.WithError(err).Warn("Failed to generate activation token")
		} else {
			frontendURL := os.Getenv("FRONTEND_URL")
			if frontendURL == "" {
				frontendURL = "http://localhost:3000"
			}
			activationLink := fmt.Sprintf("%s/activate-email?token=%s", frontendURL, token)
			if err := as.SendActivationEmail(ctx, user, activationLink); err != nil {
				as.logger.WithError(err).Error("Failed to send activation email")
			}
		}
	}

	return &SignUpResult{
		User:      user,
		IsNewUser: isNewUser,
	}, nil
}

// SignUpWithOAuth creates or links OAuth authentication
func (as *AuthService) SignUpWithOAuth(ctx context.Context, req *api.SignUpRequest) (*SignUpResult, error) {
	email := strings.ToLower(req.Email)

	// First, try to find user by email
	user, err := as.userStore.GetUserByEmail(ctx, email)
	if err != nil && !errs.IsNoSuchEntityError(err) {
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}

	isNewUser := user == nil

	if isNewUser {
		// Create new user for OAuth
		user = &interfaces.User{
			Email:      email,
			Username:   req.Username,
			FirstName:  req.FirstName,
			LastName:   req.LastName,
			AvatarURL:  req.AvatarURL,
			Provider:   req.Provider,
			ProviderID: &req.ProviderID,
			Role:       "user",
			CreatedAt:  time.Now(),
		}

		user, err = as.userStore.CreateUser(ctx, user, "", "user")
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	} else {
		// Update existing user with OAuth info if not already set
		if user.Provider == "" || user.Provider == "password" {
			updateFields := map[string]interface{}{
				"provider":    req.Provider,
				"provider_id": &req.ProviderID,
			}
			if req.AvatarURL != "" && user.AvatarURL == "" {
				updateFields["avatar_url"] = req.AvatarURL
			}

			if _, err := as.userStore.UpdateUserFields(ctx, user.ID.String(), updateFields); err != nil {
				return nil, fmt.Errorf("failed to link OAuth provider: %w", err)
			}
		}
	}

	as.logger.WithFields(logrus.Fields{
		"email":    email,
		"provider": req.Provider,
		"new":      isNewUser,
	}).Info("User signed up with OAuth")

	return &SignUpResult{
		User:      user,
		IsNewUser: isNewUser,
	}, nil
}

// AuthenticateWithPassword verifies password credentials
func (as *AuthService) AuthenticateWithPassword(ctx context.Context, email, password string) (*interfaces.User, error) {
	user, err := as.userStore.GetUserByEmail(ctx, strings.ToLower(email))
	if err != nil {
		return nil, errs.ErrNoSuchEntity
	}

	if user.Provider != "password" || user.PasswordHash == "" {
		return nil, fmt.Errorf("user does not have password authentication")
	}

	if err := encrypt.VerifyPassword(user.PasswordHash, password); err != nil {
		return nil, err
	}

	return user, nil
}

// LinkOAuthProvider links an OAuth provider to an existing user
func (as *AuthService) LinkOAuthProvider(ctx context.Context, userID string, provider, providerID string) error {
	// In a full implementation, this would create an AuthIdentity record
	// For now, update the user record
	updateFields := map[string]interface{}{
		"provider":    provider,
		"provider_id": &providerID,
	}

	_, err := as.userStore.UpdateUserFields(ctx, userID, updateFields)
	if err != nil {
		return fmt.Errorf("failed to link OAuth provider: %w", err)
	}
	return nil
}

// LogLogout records the logout event for a user
func (as *AuthService) LogLogout(ctx context.Context, username string) {
	as.logger.WithField("username", username).Info("User logged out")
}

// generateActivationToken creates a new activation token for a user
func (as *AuthService) generateActivationToken(ctx context.Context, userID string) (string, error) {
	token := uuid.New().String()
	expiration := time.Now().Add(10 * time.Minute) // Token expires in 10 minutes

	// Parse userID as UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return "", fmt.Errorf("invalid user ID: %w", err)
	}

	validationToken := &interfaces.ValidationToken{
		UserID:    userUUID,
		Token:     token,
		TokenType: "activation",
		ExpiredAt: expiration,
	}

	if err := as.userStore.CreateValidationToken(ctx, validationToken); err != nil {
		return "", fmt.Errorf("failed to create validation token: %w", err)
	}

	return token, nil
}

// generatePasswordResetToken creates a new password reset token for a user
func (as *AuthService) generatePasswordResetToken(ctx context.Context, userID string) (string, error) {
	token := uuid.New().String()
	expiration := time.Now().Add(15 * time.Minute) // Token expires in 15 minutes

	// Parse userID as UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return "", fmt.Errorf("invalid user ID: %w", err)
	}

	// First, invalidate any existing password reset tokens for this user
	// This is a simple approach - in production you might want to track them better
	validationToken := &interfaces.ValidationToken{
		UserID:    userUUID,
		Token:     token,
		TokenType: "password_reset",
		ExpiredAt: expiration,
	}

	if err := as.userStore.CreateValidationToken(ctx, validationToken); err != nil {
		return "", fmt.Errorf("failed to create password reset token: %w", err)
	}

	return token, nil
}

// SendPasswordResetEmail sends a password reset email to the user
func (as *AuthService) SendPasswordResetEmail(ctx context.Context, user *interfaces.User, resetLink string) error {
	if as.mailer == nil {
		as.logger.Warn("Mailer not configured, skipping password reset email")
		return nil
	}

	htmlContent, err := template.RenderResetPassword(resetLink)
	if err != nil {
		return fmt.Errorf("failed to render password reset email template: %w", err)
	}

	err = as.mailer.Send(ctx,
		as.mailFromName, as.mailFromEmail,
		user.FirstName+" "+user.LastName, user.Email,
		"Reset your Magic password",
		"Please reset your password by clicking the link: "+resetLink,
		htmlContent,
	)

	if err != nil {
		return fmt.Errorf("failed to send password reset email: %w", err)
	}

	as.logger.WithFields(logrus.Fields{
		"user_id": user.ID,
		"email":   user.Email,
	}).Info("Password reset email sent")

	return nil
}

// RequestPasswordReset handles forgot password request
func (as *AuthService) RequestPasswordReset(ctx context.Context, email string, baseURL string) error {
	user, err := as.userStore.GetUserByEmail(ctx, strings.ToLower(email))
	if err != nil {
		// Don't reveal if email exists or not for security
		as.logger.WithField("email", email).Info("Password reset requested for non-existent email")
		return nil
	}

	// Generate password reset token
	token, err := as.generatePasswordResetToken(ctx, user.ID.String())
	if err != nil {
		return fmt.Errorf("failed to generate password reset token: %w", err)
	}

	resetLink := fmt.Sprintf("%s/reset-password?token=%s", baseURL, token)

	// Send password reset email
	if err := as.SendPasswordResetEmail(ctx, user, resetLink); err != nil {
		return fmt.Errorf("failed to send password reset email: %w", err)
	}

	return nil
}

// ResetPassword resets the user's password using the token
func (as *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	validationToken, err := as.userStore.GetValidationToken(ctx, token)
	if err != nil {
		return fmt.Errorf("invalid or expired token: %w", err)
	}

	// Check if this is a password reset token
	if validationToken.TokenType != "password_reset" {
		return fmt.Errorf("invalid token type")
	}

	if time.Now().After(validationToken.ExpiredAt) {
		// Clean up expired token
		as.userStore.DeleteValidationToken(ctx, token)
		return fmt.Errorf("token has expired")
	}

	// Hash the new password
	hashedPassword, err := encrypt.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update user's password
	_, err = as.userStore.UpdateUserFields(ctx, validationToken.UserID.String(), map[string]interface{}{
		"password_hash": string(hashedPassword),
	})
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Delete the token after successful password reset
	if err := as.userStore.DeleteValidationToken(ctx, token); err != nil {
		as.logger.WithError(err).Warn("Failed to delete password reset token")
	}

	as.logger.WithFields(logrus.Fields{
		"user_id": validationToken.UserID,
	}).Info("Password reset successfully")

	return nil
}

// SendActivationEmail sends an activation email to the user
func (as *AuthService) SendActivationEmail(ctx context.Context, user *interfaces.User, activationLink string) error {
	if as.mailer == nil {
		as.logger.Warn("Mailer not configured, skipping activation email")
		return nil
	}

	htmlContent, err := template.RenderActivateAccount(activationLink)
	if err != nil {
		return fmt.Errorf("failed to render activation email template: %w", err)
	}

	err = as.mailer.Send(ctx,
		as.mailFromName, as.mailFromEmail,
		user.FirstName+" "+user.LastName, user.Email,
		"Activate your Magic account",
		"Please activate your account by clicking the link: "+activationLink,
		htmlContent,
	)

	if err != nil {
		return fmt.Errorf("failed to send activation email: %w", err)
	}

	as.logger.WithFields(logrus.Fields{
		"user_id": user.ID,
		"email":   user.Email,
		"from":    as.mailFromEmail,
	}).Info("Activation email sent")

	return nil
}

// ActivateAccount activates a user account using the validation token
func (as *AuthService) ActivateAccount(ctx context.Context, token string) error {
	validationToken, err := as.userStore.GetValidationToken(ctx, token)
	if err != nil {
		return fmt.Errorf("invalid or expired token: %w", err)
	}

	if time.Now().After(validationToken.ExpiredAt) {
		// Clean up expired token
		as.userStore.DeleteValidationToken(ctx, token)
		return fmt.Errorf("token has expired")
	}

	// Update user email verification status
	_, err = as.userStore.UpdateUserFields(ctx, validationToken.UserID.String(), map[string]interface{}{
		"email_verified": true,
	})
	if err != nil {
		return fmt.Errorf("failed to verify email: %w", err)
	}

	// Delete the token after successful activation
	if err := as.userStore.DeleteValidationToken(ctx, token); err != nil {
		as.logger.WithError(err).Warn("Failed to delete activation token")
	}

	as.logger.WithFields(logrus.Fields{
		"user_id": validationToken.UserID,
	}).Info("Account activated successfully")

	return nil
}
