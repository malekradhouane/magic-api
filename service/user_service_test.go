package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
)

type MockUserStore struct{ mock.Mock }

func (m *MockUserStore) CreateUser(ctx context.Context, user *interfaces.User, companyID string, role string) (*interfaces.User, error) {
	args := m.Called(ctx, user, companyID, role)
	var res *interfaces.User
	if v := args.Get(0); v != nil {
		res = v.(*interfaces.User)
	}
	return res, args.Error(1)
}
func (m *MockUserStore) Get(ctx context.Context, id string) (*interfaces.User, error) {
	args := m.Called(ctx, id)
	var res *interfaces.User
	if v := args.Get(0); v != nil {
		res = v.(*interfaces.User)
	}
	return res, args.Error(1)
}
func (m *MockUserStore) GetUserByEmail(ctx context.Context, email string) (*interfaces.User, error) {
	args := m.Called(ctx, email)
	var res *interfaces.User
	if v := args.Get(0); v != nil {
		res = v.(*interfaces.User)
	}
	return res, args.Error(1)
}
func (m *MockUserStore) GetUsers(ctx context.Context) ([]*interfaces.User, error) {
	args := m.Called(ctx)
	var res []*interfaces.User
	if v := args.Get(0); v != nil {
		res = v.([]*interfaces.User)
	}
	return res, args.Error(1)
}
func (m *MockUserStore) GetUsersByRole(ctx context.Context, role string) ([]*interfaces.User, error) {
	args := m.Called(ctx, role)
	var res []*interfaces.User
	if v := args.Get(0); v != nil {
		res = v.([]*interfaces.User)
	}
	return res, args.Error(1)
}
func (m *MockUserStore) IsEmailTaken(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}
func (m *MockUserStore) Authenticate(ctx context.Context, cred *interfaces.Credential) (*interfaces.User, error) {
	args := m.Called(ctx, cred)
	var res *interfaces.User
	if v := args.Get(0); v != nil {
		res = v.(*interfaces.User)
	}
	return res, args.Error(1)
}
func (m *MockUserStore) FindByEmailAndProvider(ctx context.Context, email, provider string) (*interfaces.User, error) {
	args := m.Called(ctx, email, provider)
	var res *interfaces.User
	if v := args.Get(0); v != nil {
		res = v.(*interfaces.User)
	}
	return res, args.Error(1)
}
func (m *MockUserStore) UpdateUser(ctx context.Context, id string, user *interfaces.User) (*interfaces.User, error) {
	args := m.Called(ctx, id, user)
	var res *interfaces.User
	if v := args.Get(0); v != nil {
		res = v.(*interfaces.User)
	}
	return res, args.Error(1)
}
func (m *MockUserStore) UpdateUserFields(ctx context.Context, id string, fields map[string]interface{}) (*interfaces.User, error) {
	args := m.Called(ctx, id, fields)
	var res *interfaces.User
	if v := args.Get(0); v != nil {
		res = v.(*interfaces.User)
	}
	return res, args.Error(1)
}
func (m *MockUserStore) DeleteUser(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserStore) CreateValidationToken(ctx context.Context, token *interfaces.ValidationToken) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}
func (m *MockUserStore) GetValidationToken(ctx context.Context, token string) (*interfaces.ValidationToken, error) {
	args := m.Called(ctx, token)
	var res *interfaces.ValidationToken
	if v := args.Get(0); v != nil {
		res = v.(*interfaces.ValidationToken)
	}
	return res, args.Error(1)
}
func (m *MockUserStore) DeleteValidationToken(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}
func (m *MockUserStore) Close() error { return nil }

// Test: Create success
func TestUserService_Create_Success(t *testing.T) {
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	email := "ok@example.com"
	mus.On("IsEmailTaken", mock.Anything, email).Return(false, nil)
	userID := uuid.New()
	// UserService.Create passes empty companyID and "user" role
	mus.On("CreateUser", mock.Anything, mock.AnythingOfType("*interfaces.User"), "", "user").Return(&interfaces.User{ID: userID, Email: email}, nil)

	res, err := svc.Create(context.Background(), &api.SignUpRequest{Email: email, Password: "pwd", Role: "user"})
	assert.NoError(t, err)
	assert.Equal(t, userID.String(), res.ID)
	assert.Equal(t, email, res.Email)
	mus.AssertExpectations(t)
}

// Test: GetUser not found
func TestUserService_GetUser_NotFound(t *testing.T) {
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	mus.On("Get", mock.Anything, "id-404").Return((*interfaces.User)(nil), errs.ErrNoSuchEntity)

	_, err := svc.GetUser(context.Background(), "id-404")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNoSuchEntity))
	mus.AssertExpectations(t)
}

// Test: DeleteUser success
func TestUserService_DeleteUser_Success(t *testing.T) {
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	mus.On("DeleteUser", mock.Anything, "id-1").Return(nil)

	err := svc.DeleteUser(context.Background(), "id-1")
	assert.NoError(t, err)
	mus.AssertExpectations(t)
}

// Test: UpdateUserFields success
func TestUserService_UpdateUserFields_Success(t *testing.T) {
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	fields := map[string]interface{}{"first_name": "John"}
	uid := uuid.New()
	mus.On("UpdateUserFields", mock.Anything, "id-1", fields).Return(&interfaces.User{ID: uid}, nil)

	res, err := svc.UpdateUserFields(context.Background(), "id-1", fields)
	assert.NoError(t, err)
	assert.Equal(t, uid, res.ID)
	mus.AssertExpectations(t)
}

// Test: Create email already taken
func TestUserService_Create_EmailTaken(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	mus.On("IsEmailTaken", mock.Anything, "taken@example.com").Return(true, nil)

	_, err := svc.Create(context.Background(), &api.SignUpRequest{
		Email: "TAKEN@example.com", Password: "password",
	})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrEmailTaken))
}

// Test: Create store error on IsEmailTaken
func TestUserService_Create_IsEmailTakenError(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	mus.On("IsEmailTaken", mock.Anything, "err@example.com").
		Return(false, errors.New("db error"))

	_, err := svc.Create(context.Background(), &api.SignUpRequest{
		Email: "err@example.com", Password: "password",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// Test: GetUser success
func TestUserService_GetUser_Success(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	uid := uuid.New()
	mus.On("Get", mock.Anything, uid.String()).
		Return(&interfaces.User{ID: uid, Email: "ok@test.com"}, nil)

	user, err := svc.GetUser(context.Background(), uid.String())
	assert.NoError(t, err)
	assert.Equal(t, "ok@test.com", user.Email)
}

// Test: GetUser generic error (not ErrNoSuchEntity)
func TestUserService_GetUser_GenericError(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	mus.On("Get", mock.Anything, "id-err").
		Return((*interfaces.User)(nil), errors.New("db error"))

	_, err := svc.GetUser(context.Background(), "id-err")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// Test: GetUsers success
func TestUserService_GetUsers_Success(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	mus.On("GetUsers", mock.Anything).Return([]*interfaces.User{
		{Email: "a@test.com"}, {Email: "b@test.com"},
	}, nil)

	users, err := svc.GetUsers(context.Background())
	assert.NoError(t, err)
	assert.Len(t, users, 2)
}

// Test: GetUsers error
func TestUserService_GetUsers_Error(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	mus.On("GetUsers", mock.Anything).
		Return(([]*interfaces.User)(nil), errors.New("db error"))

	_, err := svc.GetUsers(context.Background())
	assert.Error(t, err)
}

// Test: GetUsersByRole
func TestUserService_GetUsersByRole_Success(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	mus.On("GetUsersByRole", mock.Anything, "admin").Return([]*interfaces.User{
		{Email: "admin@test.com", Role: "admin"},
	}, nil)

	users, err := svc.GetUsersByRole(context.Background(), "admin")
	assert.NoError(t, err)
	assert.Len(t, users, 1)
}

// Test: DeleteUser not found
func TestUserService_DeleteUser_NotFound(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	mus.On("DeleteUser", mock.Anything, "missing").Return(errs.ErrNoSuchEntity)

	err := svc.DeleteUser(context.Background(), "missing")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNoSuchEntity))
}

// Test: DeleteUser generic error
func TestUserService_DeleteUser_GenericError(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	mus.On("DeleteUser", mock.Anything, "err").Return(errors.New("db error"))

	err := svc.DeleteUser(context.Background(), "err")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete user")
}

// Test: UpdateUser not found
func TestUserService_UpdateUser_NotFound(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	mus.On("UpdateUser", mock.Anything, "missing", mock.Anything).
		Return((*interfaces.User)(nil), errs.ErrNoSuchEntity)

	_, err := svc.UpdateUser(context.Background(), "missing", &api.UpdateUserRequest{
		FirstName: "John",
	})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNoSuchEntity))
}

// Test: UpdateUser with phone normalization
func TestUserService_UpdateUser_PhoneNormalization(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	uid := uuid.New()
	mus.On("UpdateUser", mock.Anything, uid.String(), mock.Anything).
		Return(&interfaces.User{ID: uid, PhoneNumber: "+21622334455"}, nil)

	got, err := svc.UpdateUser(context.Background(), uid.String(), &api.UpdateUserRequest{
		PhoneNumber: "22334455",
	})
	assert.NoError(t, err)
	assert.Equal(t, "+21622334455", got.PhoneNumber)
}

// Test: UpdateUser invalid phone
func TestUserService_UpdateUser_InvalidPhone(t *testing.T) {
	t.Parallel()
	svc := NewUserService(new(MockUserStore), nil)

	_, err := svc.UpdateUser(context.Background(), uuid.New().String(), &api.UpdateUserRequest{
		PhoneNumber: "123", // invalid Tunisian phone
	})
	assert.Error(t, err)
}

// Test: UpdateUserFields phone normalization
func TestUserService_UpdateUserFields_PhoneNormalization(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	uid := uuid.New()
	mus.On("UpdateUserFields", mock.Anything, uid.String(), mock.MatchedBy(func(f map[string]interface{}) bool {
		return f["phone_number"] == "+21622334455"
	})).Return(&interfaces.User{ID: uid}, nil)

	fields := map[string]interface{}{"phone_number": "22334455"}
	_, err := svc.UpdateUserFields(context.Background(), uid.String(), fields)
	assert.NoError(t, err)
}

// Test: UpdateUserFields invalid phone
func TestUserService_UpdateUserFields_InvalidPhone(t *testing.T) {
	t.Parallel()
	svc := NewUserService(new(MockUserStore), nil)

	fields := map[string]interface{}{"phone_number": "123"}
	_, err := svc.UpdateUserFields(context.Background(), uuid.New().String(), fields)
	assert.Error(t, err)
}

// Test: UpdateUserFields not found
func TestUserService_UpdateUserFields_NotFound(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	mus.On("UpdateUserFields", mock.Anything, "missing", mock.Anything).
		Return((*interfaces.User)(nil), errs.ErrNoSuchEntity)

	_, err := svc.UpdateUserFields(context.Background(), "missing", map[string]interface{}{"first_name": "X"})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNoSuchEntity))
}

// Test: CreateWithPassword success
func TestUserService_CreateWithPassword_Success(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	email := "new@test.com"
	uid := uuid.New()
	mus.On("IsEmailTaken", mock.Anything, email).Return(false, nil)
	mus.On("CreateUser", mock.Anything, mock.AnythingOfType("*interfaces.User"), "", mock.Anything).
		Return(&interfaces.User{ID: uid, Email: email}, nil)

	res, err := svc.CreateWithPassword(context.Background(), &api.SignUpRequest{
		Email:    email,
		Password: "mypassword",
		Role:     "user",
	})
	assert.NoError(t, err)
	assert.Equal(t, uid.String(), res.ID)
}

// Test: CreateWithPassword email taken
func TestUserService_CreateWithPassword_EmailTaken(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	mus.On("IsEmailTaken", mock.Anything, "taken@test.com").Return(true, nil)

	_, err := svc.CreateWithPassword(context.Background(), &api.SignUpRequest{
		Email:    "TAKEN@test.com",
		Password: "password",
	})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrEmailTaken))
}

// Test: CreateOrGetOAuthUser returns existing
func TestUserService_CreateOrGetOAuthUser_ExistingUser(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	uid := uuid.New()
	mus.On("FindByEmailAndProvider", mock.Anything, "oauth@test.com", "google").
		Return(&interfaces.User{ID: uid, Email: "oauth@test.com"}, nil)

	user, err := svc.CreateOrGetOAuthUser(context.Background(), &api.SignUpRequest{
		Email:    "OAUTH@test.com",
		Provider: "google",
	})
	assert.NoError(t, err)
	assert.Equal(t, uid, user.ID)
}

// Test: CreateOrGetOAuthUser creates new
func TestUserService_CreateOrGetOAuthUser_NewUser(t *testing.T) {
	t.Parallel()
	mus := new(MockUserStore)
	svc := NewUserService(mus, nil)

	mus.On("FindByEmailAndProvider", mock.Anything, "new@test.com", "google").
		Return((*interfaces.User)(nil), errors.New("not found"))
	uid := uuid.New()
	mus.On("CreateUser", mock.Anything, mock.AnythingOfType("*interfaces.User"), "", "").
		Return(&interfaces.User{ID: uid, Email: "new@test.com"}, nil)

	user, err := svc.CreateOrGetOAuthUser(context.Background(), &api.SignUpRequest{
		Email:    "new@test.com",
		Provider: "google",
	})
	assert.NoError(t, err)
	assert.Equal(t, uid, user.ID)
}

// Test: NewUserService nil logger
func TestNewUserService_NilLogger(t *testing.T) {
	t.Parallel()

	svc := NewUserService(new(MockUserStore), nil)
	assert.NotNil(t, svc)
	assert.NotNil(t, svc.logger)
}
