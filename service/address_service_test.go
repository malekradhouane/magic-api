package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
)

// ---------------------------------------------------------------------------
// AddressService.ListByUser
// ---------------------------------------------------------------------------

func TestAddressService_ListByUser_Success(t *testing.T) {
	t.Parallel()
	store := new(MockAddressStore)
	svc := NewAddressService(store, nil)

	uid := uuid.New().String()
	store.On("ListByUserID", mock.Anything, uid).
		Return([]*interfaces.Address{{Label: "Domicile"}}, nil)

	got, err := svc.ListByUser(context.Background(), uid)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestAddressService_ListByUser_EmptyUserID(t *testing.T) {
	t.Parallel()
	svc := NewAddressService(new(MockAddressStore), nil)

	_, err := svc.ListByUser(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user id is required")
}

// ---------------------------------------------------------------------------
// AddressService.Create
// ---------------------------------------------------------------------------

func TestAddressService_Create_Success(t *testing.T) {
	t.Parallel()
	store := new(MockAddressStore)
	svc := NewAddressService(store, nil)

	uid := uuid.New()
	store.On("Create", mock.Anything, mock.AnythingOfType("*interfaces.Address")).
		Return(&interfaces.Address{ID: uuid.New(), UserID: uid, Label: "Domicile"}, nil)

	got, err := svc.Create(context.Background(), uid.String(), &api.CreateAddressRequest{
		FirstName:   "John",
		LastName:    "Doe",
		Phone:       "+21622334455",
		Gouvernorat: "Tunis",
		Address:     "123 Rue Principale",
	})
	require.NoError(t, err)
	assert.Equal(t, "Domicile", got.Label)
}

func TestAddressService_Create_DefaultLabel(t *testing.T) {
	t.Parallel()
	store := new(MockAddressStore)
	svc := NewAddressService(store, nil)

	uid := uuid.New()
	store.On("Create", mock.Anything, mock.MatchedBy(func(a *interfaces.Address) bool {
		return a.Label == "Domicile"
	})).Return(&interfaces.Address{Label: "Domicile"}, nil)

	_, err := svc.Create(context.Background(), uid.String(), &api.CreateAddressRequest{
		Label:       "", // empty -> defaults to "Domicile"
		FirstName:   "Jane",
		LastName:    "Doe",
		Phone:       "+21622334455",
		Gouvernorat: "Sfax",
		Address:     "456 Rue",
	})
	require.NoError(t, err)
}

func TestAddressService_Create_InvalidUserID(t *testing.T) {
	t.Parallel()
	svc := NewAddressService(new(MockAddressStore), nil)

	_, err := svc.Create(context.Background(), "bad", &api.CreateAddressRequest{
		FirstName:   "John",
		LastName:    "Doe",
		Phone:       "+21622334455",
		Gouvernorat: "Tunis",
		Address:     "Rue",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid user id")
}

func TestAddressService_Create_MissingFields(t *testing.T) {
	t.Parallel()
	svc := NewAddressService(new(MockAddressStore), nil)
	uid := uuid.New()

	_, err := svc.Create(context.Background(), uid.String(), &api.CreateAddressRequest{
		FirstName:   "", // missing
		LastName:    "Doe",
		Phone:       "+21622334455",
		Gouvernorat: "Tunis",
		Address:     "Rue",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all address fields are required")
}

func TestAddressService_Create_NilRequest(t *testing.T) {
	t.Parallel()
	svc := NewAddressService(new(MockAddressStore), nil)
	uid := uuid.New()

	_, err := svc.Create(context.Background(), uid.String(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request is required")
}

// ---------------------------------------------------------------------------
// AddressService.Update
// ---------------------------------------------------------------------------

func TestAddressService_Update_Success(t *testing.T) {
	t.Parallel()
	store := new(MockAddressStore)
	svc := NewAddressService(store, nil)

	uid := uuid.New().String()
	addrID := uuid.New().String()
	existing := &interfaces.Address{
		Label:       "Domicile",
		FirstName:   "John",
		LastName:    "Doe",
		Phone:       "+21622334455",
		Gouvernorat: "Tunis",
		Address:     "Rue 1",
	}
	store.On("GetByID", mock.Anything, uid, addrID).Return(existing, nil)
	store.On("Update", mock.Anything, mock.AnythingOfType("*interfaces.Address")).
		Return(existing, nil)

	newLabel := "Bureau"
	got, err := svc.Update(context.Background(), uid, addrID, &api.UpdateAddressRequest{
		Label: &newLabel,
	})
	require.NoError(t, err)
	assert.Equal(t, "Bureau", got.Label)
}

func TestAddressService_Update_NotFound(t *testing.T) {
	t.Parallel()
	store := new(MockAddressStore)
	svc := NewAddressService(store, nil)

	store.On("GetByID", mock.Anything, "u1", "a1").
		Return((*interfaces.Address)(nil), errs.ErrNoSuchEntity)

	_, err := svc.Update(context.Background(), "u1", "a1", &api.UpdateAddressRequest{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNoSuchEntity))
}

func TestAddressService_Update_EmptyFieldsRejected(t *testing.T) {
	t.Parallel()
	store := new(MockAddressStore)
	svc := NewAddressService(store, nil)

	uid := uuid.New().String()
	addrID := uuid.New().String()
	existing := &interfaces.Address{
		Label:       "Domicile",
		FirstName:   "John",
		LastName:    "Doe",
		Phone:       "+21622334455",
		Gouvernorat: "Tunis",
		Address:     "Rue 1",
	}
	store.On("GetByID", mock.Anything, uid, addrID).Return(existing, nil)

	emptyName := ""
	_, err := svc.Update(context.Background(), uid, addrID, &api.UpdateAddressRequest{
		FirstName: &emptyName,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "address fields cannot be empty")
}

// ---------------------------------------------------------------------------
// AddressService.Delete
// ---------------------------------------------------------------------------

func TestAddressService_Delete_Success(t *testing.T) {
	t.Parallel()
	store := new(MockAddressStore)
	svc := NewAddressService(store, nil)

	store.On("Delete", mock.Anything, "u1", "a1").Return(nil)

	err := svc.Delete(context.Background(), "u1", "a1")
	require.NoError(t, err)
}

func TestAddressService_Delete_NotFound(t *testing.T) {
	t.Parallel()
	store := new(MockAddressStore)
	svc := NewAddressService(store, nil)

	store.On("Delete", mock.Anything, "u1", "a1").Return(errs.ErrNoSuchEntity)

	err := svc.Delete(context.Background(), "u1", "a1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNoSuchEntity))
}

// ---------------------------------------------------------------------------
// AddressService.SetDefault
// ---------------------------------------------------------------------------

func TestAddressService_SetDefault_Success(t *testing.T) {
	t.Parallel()
	store := new(MockAddressStore)
	svc := NewAddressService(store, nil)

	store.On("SetDefault", mock.Anything, "u1", "a1").Return(nil)

	err := svc.SetDefault(context.Background(), "u1", "a1")
	require.NoError(t, err)
}

func TestAddressService_SetDefault_NotFound(t *testing.T) {
	t.Parallel()
	store := new(MockAddressStore)
	svc := NewAddressService(store, nil)

	store.On("SetDefault", mock.Anything, "u1", "a1").Return(errs.ErrNoSuchEntity)

	err := svc.SetDefault(context.Background(), "u1", "a1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNoSuchEntity))
}

// ---------------------------------------------------------------------------
// AddressService.SaveFromShipping
// ---------------------------------------------------------------------------

func TestAddressService_SaveFromShipping_EmptyUserID(t *testing.T) {
	t.Parallel()
	svc := NewAddressService(new(MockAddressStore), nil)

	err := svc.SaveFromShipping(context.Background(), "", interfaces.ShippingInfo{
		Phone: "+21622334455", Gouvernorat: "Tunis", Address: "Rue",
	})
	require.NoError(t, err) // silently returns nil
}

func TestAddressService_SaveFromShipping_InvalidUserID(t *testing.T) {
	t.Parallel()
	svc := NewAddressService(new(MockAddressStore), nil)

	err := svc.SaveFromShipping(context.Background(), "bad-uuid", interfaces.ShippingInfo{
		Phone: "+21622334455", Gouvernorat: "Tunis", Address: "Rue",
	})
	require.NoError(t, err) // silently returns nil for invalid UUIDs
}

func TestAddressService_SaveFromShipping_MissingFields(t *testing.T) {
	t.Parallel()
	svc := NewAddressService(new(MockAddressStore), nil)

	uid := uuid.New().String()
	err := svc.SaveFromShipping(context.Background(), uid, interfaces.ShippingInfo{
		Phone: "", Gouvernorat: "", Address: "",
	})
	require.NoError(t, err) // silently returns nil when fields are missing
}

func TestAddressService_SaveFromShipping_NewAddress(t *testing.T) {
	t.Parallel()
	store := new(MockAddressStore)
	svc := NewAddressService(store, nil)

	uid := uuid.New().String()
	store.On("FindMatching", mock.Anything, uid, mock.Anything, "Tunis", "Rue 1").
		Return((*interfaces.Address)(nil), errs.ErrNoSuchEntity)
	store.On("Create", mock.Anything, mock.AnythingOfType("*interfaces.Address")).
		Return(&interfaces.Address{Label: "Livraison"}, nil)

	err := svc.SaveFromShipping(context.Background(), uid, interfaces.ShippingInfo{
		FirstName: "John", LastName: "Doe",
		Phone: "+21622334455", Gouvernorat: "Tunis", Address: "Rue 1",
	})
	require.NoError(t, err)
	store.AssertExpectations(t)
}

func TestAddressService_SaveFromShipping_ExistingAddress(t *testing.T) {
	t.Parallel()
	store := new(MockAddressStore)
	svc := NewAddressService(store, nil)

	uid := uuid.New().String()
	existing := &interfaces.Address{Label: "Old", FirstName: "OldName"}
	store.On("FindMatching", mock.Anything, uid, mock.Anything, "Tunis", "Rue 1").
		Return(existing, nil)
	store.On("Update", mock.Anything, mock.AnythingOfType("*interfaces.Address")).
		Return(existing, nil)

	err := svc.SaveFromShipping(context.Background(), uid, interfaces.ShippingInfo{
		FirstName: "NewName", LastName: "Doe",
		Phone: "+21622334455", Gouvernorat: "Tunis", Address: "Rue 1",
	})
	require.NoError(t, err)
	assert.Equal(t, "NewName", existing.FirstName)
}

// ---------------------------------------------------------------------------
// validateAddressRequest
// ---------------------------------------------------------------------------

func TestValidateAddressRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     *api.CreateAddressRequest
		wantErr bool
	}{
		{name: "nil request", req: nil, wantErr: true},
		{name: "empty first name", req: &api.CreateAddressRequest{FirstName: "", LastName: "D", Phone: "p", Gouvernorat: "g", Address: "a"}, wantErr: true},
		{name: "valid", req: &api.CreateAddressRequest{FirstName: "J", LastName: "D", Phone: "p", Gouvernorat: "g", Address: "a"}, wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateAddressRequest(tc.req)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// normalizePhone (best-effort helper)
// ---------------------------------------------------------------------------

func TestNormalizePhoneHelper(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"+21622334455", "+21622334455"},
		{"22334455", "+21622334455"},
		{"invalid", "invalid"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := normalizePhone(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}
