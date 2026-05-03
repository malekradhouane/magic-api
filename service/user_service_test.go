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
	// UserService.Create passes empty strings for companyID and role
	mus.On("CreateUser", mock.Anything, mock.AnythingOfType("*interfaces.User"), "", "").Return(&interfaces.User{ID: userID, Email: email}, nil)

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
