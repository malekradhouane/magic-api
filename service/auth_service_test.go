package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
)

// ---------------------------------------------------------------------------
// AuthService.AuthenticateWithPassword
// ---------------------------------------------------------------------------

func TestAuthService_AuthenticateWithPassword_UserNotFound(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userStore.On("GetUserByEmail", mock.Anything, "nobody@test.com").
		Return((*interfaces.User)(nil), errs.ErrNoSuchEntity)

	_, err := svc.AuthenticateWithPassword(context.Background(), "nobody@test.com", "password")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNoSuchEntity))
}

func TestAuthService_AuthenticateWithPassword_NoPasswordProvider(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userStore.On("GetUserByEmail", mock.Anything, "oauth@test.com").
		Return(&interfaces.User{
			Email:    "oauth@test.com",
			Provider: "google",
		}, nil)

	_, err := svc.AuthenticateWithPassword(context.Background(), "oauth@test.com", "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not have password authentication")
}

func TestAuthService_AuthenticateWithPassword_EmptyPasswordHash(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userStore.On("GetUserByEmail", mock.Anything, "nohash@test.com").
		Return(&interfaces.User{
			Email:        "nohash@test.com",
			Provider:     "password",
			PasswordHash: "",
		}, nil)

	_, err := svc.AuthenticateWithPassword(context.Background(), "nohash@test.com", "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not have password authentication")
}

// ---------------------------------------------------------------------------
// AuthService.SignUpWithPassword
// ---------------------------------------------------------------------------

func TestAuthService_SignUpWithPassword_EmailTaken(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userStore.On("GetUserByEmail", mock.Anything, "taken@test.com").
		Return(&interfaces.User{Email: "taken@test.com"}, nil)

	_, err := svc.SignUpWithPassword(context.Background(), &api.SignUpRequest{
		Email:    "TAKEN@test.com",
		Password: "password123",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrEmailTaken))
}

func TestAuthService_SignUpWithPassword_DefaultRole(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userStore.On("GetUserByEmail", mock.Anything, "new@test.com").
		Return((*interfaces.User)(nil), errs.ErrNoSuchEntity)
	userStore.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *interfaces.User) bool {
		return u.Role == "user"
	}), "", "user").Return(&interfaces.User{
		ID: uuid.New(), Email: "new@test.com", Role: "user",
	}, nil)
	userStore.On("UpdateUserFields", mock.Anything, mock.Anything, mock.Anything).
		Return(&interfaces.User{}, nil)
	userStore.On("CreateValidationToken", mock.Anything, mock.Anything).Return(nil)

	result, err := svc.SignUpWithPassword(context.Background(), &api.SignUpRequest{
		Email:    "new@test.com",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.True(t, result.IsNewUser)
}

func TestAuthService_SignUpWithPassword_ExplicitAdminRole(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userStore.On("GetUserByEmail", mock.Anything, "admin@test.com").
		Return((*interfaces.User)(nil), errs.ErrNoSuchEntity)
	userStore.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *interfaces.User) bool {
		return u.Role == "admin"
	}), "", "admin").Return(&interfaces.User{
		ID: uuid.New(), Email: "admin@test.com", Role: "admin",
	}, nil)
	userStore.On("UpdateUserFields", mock.Anything, mock.Anything, mock.Anything).
		Return(&interfaces.User{}, nil)
	userStore.On("CreateValidationToken", mock.Anything, mock.Anything).Return(nil)

	result, err := svc.SignUpWithPassword(context.Background(), &api.SignUpRequest{
		Email:    "admin@test.com",
		Password: "password123",
		Role:     "admin",
	})
	require.NoError(t, err)
	assert.NotNil(t, result.User)
}

func TestAuthService_SignUpWithPassword_DefaultUsername(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userStore.On("GetUserByEmail", mock.Anything, "noname@test.com").
		Return((*interfaces.User)(nil), errs.ErrNoSuchEntity)
	userStore.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *interfaces.User) bool {
		return u.Username == "noname@test.com" // username defaults to email
	}), "", "user").Return(&interfaces.User{
		ID: uuid.New(), Email: "noname@test.com", Username: "noname@test.com",
	}, nil)
	userStore.On("UpdateUserFields", mock.Anything, mock.Anything, mock.Anything).
		Return(&interfaces.User{}, nil)
	userStore.On("CreateValidationToken", mock.Anything, mock.Anything).Return(nil)

	_, err := svc.SignUpWithPassword(context.Background(), &api.SignUpRequest{
		Email:    "noname@test.com",
		Password: "password123",
		Username: "", // empty
	})
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// AuthService.SignUpWithOAuth
// ---------------------------------------------------------------------------

func TestAuthService_SignUpWithOAuth_NewUser(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userStore.On("GetUserByEmail", mock.Anything, "oauth@test.com").
		Return((*interfaces.User)(nil), errs.ErrNoSuchEntity)
	userStore.On("CreateUser", mock.Anything, mock.AnythingOfType("*interfaces.User"), "", "user").
		Return(&interfaces.User{
			ID: uuid.New(), Email: "oauth@test.com", Provider: "google",
		}, nil)

	result, err := svc.SignUpWithOAuth(context.Background(), &api.SignUpRequest{
		Email:      "oauth@test.com",
		Provider:   "google",
		ProviderID: "google-123",
		FirstName:  "John",
	})
	require.NoError(t, err)
	assert.True(t, result.IsNewUser)
}

func TestAuthService_SignUpWithOAuth_ExistingUser_LinkProvider(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	existingID := uuid.New()
	userStore.On("GetUserByEmail", mock.Anything, "existing@test.com").
		Return(&interfaces.User{
			ID:       existingID,
			Email:    "existing@test.com",
			Provider: "password",
		}, nil)
	userStore.On("UpdateUserFields", mock.Anything, existingID.String(), mock.Anything).
		Return(&interfaces.User{}, nil)

	result, err := svc.SignUpWithOAuth(context.Background(), &api.SignUpRequest{
		Email:      "existing@test.com",
		Provider:   "google",
		ProviderID: "google-456",
	})
	require.NoError(t, err)
	assert.False(t, result.IsNewUser)
}

func TestAuthService_SignUpWithOAuth_ExistingUser_AlreadyOAuth(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	existingID := uuid.New()
	userStore.On("GetUserByEmail", mock.Anything, "google@test.com").
		Return(&interfaces.User{
			ID:       existingID,
			Email:    "google@test.com",
			Provider: "google",
		}, nil)

	result, err := svc.SignUpWithOAuth(context.Background(), &api.SignUpRequest{
		Email:      "google@test.com",
		Provider:   "google",
		ProviderID: "google-789",
	})
	require.NoError(t, err)
	assert.False(t, result.IsNewUser)
	// UpdateUserFields should NOT be called since provider is already set.
	userStore.AssertNotCalled(t, "UpdateUserFields")
}

// ---------------------------------------------------------------------------
// AuthService.LinkOAuthProvider
// ---------------------------------------------------------------------------

func TestAuthService_LinkOAuthProvider_Success(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userStore.On("UpdateUserFields", mock.Anything, "user-1", mock.Anything).
		Return(&interfaces.User{}, nil)

	err := svc.LinkOAuthProvider(context.Background(), "user-1", "github", "gh-123")
	require.NoError(t, err)
}

func TestAuthService_LinkOAuthProvider_StoreError(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userStore.On("UpdateUserFields", mock.Anything, "user-1", mock.Anything).
		Return((*interfaces.User)(nil), errors.New("db error"))

	err := svc.LinkOAuthProvider(context.Background(), "user-1", "github", "gh-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to link OAuth provider")
}

// ---------------------------------------------------------------------------
// AuthService.LogLogout
// ---------------------------------------------------------------------------

func TestAuthService_LogLogout_NoError(t *testing.T) {
	t.Parallel()
	svc := NewAuthService(new(MockUserStore), nil, nil, "", "")

	// Should not panic.
	svc.LogLogout(context.Background(), "user@test.com")
}

// ---------------------------------------------------------------------------
// AuthService.ResetPassword
// ---------------------------------------------------------------------------

func TestAuthService_ResetPassword_InvalidToken(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userStore.On("GetValidationToken", mock.Anything, "bad-token").
		Return((*interfaces.ValidationToken)(nil), errors.New("not found"))

	err := svc.ResetPassword(context.Background(), "bad-token", "newpassword")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired token")
}

func TestAuthService_ResetPassword_WrongTokenType(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userStore.On("GetValidationToken", mock.Anything, "activation-token").
		Return(&interfaces.ValidationToken{
			Token:     "activation-token",
			TokenType: "activation",
			ExpiredAt: time.Now().Add(10 * time.Minute),
		}, nil)

	err := svc.ResetPassword(context.Background(), "activation-token", "newpassword")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token type")
}

func TestAuthService_ResetPassword_ExpiredToken(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userStore.On("GetValidationToken", mock.Anything, "expired-token").
		Return(&interfaces.ValidationToken{
			Token:     "expired-token",
			TokenType: "password_reset",
			ExpiredAt: time.Now().Add(-1 * time.Hour), // expired
		}, nil)
	userStore.On("DeleteValidationToken", mock.Anything, "expired-token").Return(nil)

	err := svc.ResetPassword(context.Background(), "expired-token", "newpassword")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token has expired")
}

func TestAuthService_ResetPassword_Success(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userID := uuid.New()
	userStore.On("GetValidationToken", mock.Anything, "valid-token").
		Return(&interfaces.ValidationToken{
			UserID:    userID,
			Token:     "valid-token",
			TokenType: "password_reset",
			ExpiredAt: time.Now().Add(10 * time.Minute),
		}, nil)
	userStore.On("UpdateUserFields", mock.Anything, userID.String(), mock.Anything).
		Return(&interfaces.User{}, nil)
	userStore.On("DeleteValidationToken", mock.Anything, "valid-token").Return(nil)

	err := svc.ResetPassword(context.Background(), "valid-token", "newpassword123")
	require.NoError(t, err)
	userStore.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// AuthService.ActivateAccount
// ---------------------------------------------------------------------------

func TestAuthService_ActivateAccount_InvalidToken(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userStore.On("GetValidationToken", mock.Anything, "bad").
		Return((*interfaces.ValidationToken)(nil), errors.New("not found"))

	err := svc.ActivateAccount(context.Background(), "bad")
	require.Error(t, err)
}

func TestAuthService_ActivateAccount_ExpiredToken(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userStore.On("GetValidationToken", mock.Anything, "expired").
		Return(&interfaces.ValidationToken{
			Token:     "expired",
			TokenType: "activation",
			ExpiredAt: time.Now().Add(-1 * time.Hour),
		}, nil)
	userStore.On("DeleteValidationToken", mock.Anything, "expired").Return(nil)

	err := svc.ActivateAccount(context.Background(), "expired")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token has expired")
}

func TestAuthService_ActivateAccount_Success(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userID := uuid.New()
	userStore.On("GetValidationToken", mock.Anything, "activate").
		Return(&interfaces.ValidationToken{
			UserID:    userID,
			Token:     "activate",
			TokenType: "activation",
			ExpiredAt: time.Now().Add(10 * time.Minute),
		}, nil)
	userStore.On("UpdateUserFields", mock.Anything, userID.String(), mock.MatchedBy(func(f map[string]interface{}) bool {
		return f["email_verified"] == true
	})).Return(&interfaces.User{}, nil)
	userStore.On("DeleteValidationToken", mock.Anything, "activate").Return(nil)

	err := svc.ActivateAccount(context.Background(), "activate")
	require.NoError(t, err)
	userStore.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// AuthService.SendActivationEmail
// ---------------------------------------------------------------------------

func TestAuthService_SendActivationEmail_NilMailer(t *testing.T) {
	t.Parallel()
	svc := NewAuthService(new(MockUserStore), nil, nil, "", "")

	err := svc.SendActivationEmail(context.Background(), &interfaces.User{
		Email: "test@test.com",
	}, "http://example.com/activate")
	require.NoError(t, err) // should silently succeed
}

func TestAuthService_SendActivationEmail_Success(t *testing.T) {
	t.Parallel()
	mailer := new(MockMailer)
	svc := NewAuthService(new(MockUserStore), nil, mailer, "Magic", "noreply@magic.fr")

	mailer.On("Send", mock.Anything, "Magic", "noreply@magic.fr",
		mock.Anything, "test@test.com", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	err := svc.SendActivationEmail(context.Background(), &interfaces.User{
		Email: "test@test.com",
	}, "http://example.com/activate")
	require.NoError(t, err)
	mailer.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// AuthService.SendPasswordResetEmail
// ---------------------------------------------------------------------------

func TestAuthService_SendPasswordResetEmail_NilMailer(t *testing.T) {
	t.Parallel()
	svc := NewAuthService(new(MockUserStore), nil, nil, "", "")

	err := svc.SendPasswordResetEmail(context.Background(), &interfaces.User{
		Email: "test@test.com",
	}, "http://example.com/reset")
	require.NoError(t, err) // should silently succeed
}

// ---------------------------------------------------------------------------
// AuthService.RequestPasswordReset
// ---------------------------------------------------------------------------

func TestAuthService_RequestPasswordReset_NonExistentEmail(t *testing.T) {
	t.Parallel()
	userStore := new(MockUserStore)
	svc := NewAuthService(userStore, nil, nil, "", "")

	userStore.On("GetUserByEmail", mock.Anything, "nobody@test.com").
		Return((*interfaces.User)(nil), errs.ErrNoSuchEntity)

	// Should NOT return error (security: don't reveal if email exists).
	err := svc.RequestPasswordReset(context.Background(), "nobody@test.com", "http://example.com")
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// NewAuthService defaults
// ---------------------------------------------------------------------------

func TestNewAuthService_Defaults(t *testing.T) {
	t.Parallel()

	svc := NewAuthService(new(MockUserStore), nil, nil, "", "")
	assert.Equal(t, "Magic", svc.mailFromName)
	assert.Equal(t, "noreply@magic.fr", svc.mailFromEmail)
}

func TestNewAuthService_CustomFrom(t *testing.T) {
	t.Parallel()

	svc := NewAuthService(new(MockUserStore), nil, nil, "MyBrand", "hello@brand.com")
	assert.Equal(t, "MyBrand", svc.mailFromName)
	assert.Equal(t, "hello@brand.com", svc.mailFromEmail)
}
