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
// CategoryService.Create
// ---------------------------------------------------------------------------

func TestCategoryService_Create_Success(t *testing.T) {
	t.Parallel()
	store := new(MockCategoryStore)
	svc := NewCategoryService(store, nil)

	catID := uuid.New()
	store.On("Create", mock.Anything, mock.AnythingOfType("*interfaces.Category")).
		Return(&interfaces.Category{ID: catID, Name: "Chemises", Slug: "chemises"}, nil)

	got, err := svc.Create(context.Background(), &api.CreateCategoryRequest{
		Name:     "Chemises",
		IsActive: true,
	})
	require.NoError(t, err)
	assert.Equal(t, catID, got.ID)
	assert.Equal(t, "Chemises", got.Name)
	store.AssertExpectations(t)
}

func TestCategoryService_Create_WithParentID(t *testing.T) {
	t.Parallel()
	store := new(MockCategoryStore)
	svc := NewCategoryService(store, nil)

	parentID := uuid.New().String()
	store.On("Create", mock.Anything, mock.AnythingOfType("*interfaces.Category")).
		Return(&interfaces.Category{ID: uuid.New(), Name: "Sous-cat"}, nil)

	got, err := svc.Create(context.Background(), &api.CreateCategoryRequest{
		Name:     "Sous-cat",
		ParentID: &parentID,
	})
	require.NoError(t, err)
	assert.NotNil(t, got)
	store.AssertExpectations(t)
}

func TestCategoryService_Create_InvalidParentID(t *testing.T) {
	t.Parallel()
	store := new(MockCategoryStore)
	svc := NewCategoryService(store, nil)

	bad := "not-a-uuid"
	_, err := svc.Create(context.Background(), &api.CreateCategoryRequest{
		Name:     "Bad",
		ParentID: &bad,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid parent_id")
}

func TestCategoryService_Create_StoreError(t *testing.T) {
	t.Parallel()
	store := new(MockCategoryStore)
	svc := NewCategoryService(store, nil)

	store.On("Create", mock.Anything, mock.Anything).
		Return((*interfaces.Category)(nil), errors.New("db error"))

	_, err := svc.Create(context.Background(), &api.CreateCategoryRequest{Name: "Fail"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// ---------------------------------------------------------------------------
// CategoryService.Get
// ---------------------------------------------------------------------------

func TestCategoryService_Get_Success(t *testing.T) {
	t.Parallel()
	store := new(MockCategoryStore)
	svc := NewCategoryService(store, nil)

	catID := uuid.New()
	store.On("GetByID", mock.Anything, catID.String()).
		Return(&interfaces.Category{ID: catID, Name: "Pantalons"}, nil)

	got, err := svc.Get(context.Background(), catID.String())
	require.NoError(t, err)
	assert.Equal(t, "Pantalons", got.Name)
}

func TestCategoryService_Get_NotFound(t *testing.T) {
	t.Parallel()
	store := new(MockCategoryStore)
	svc := NewCategoryService(store, nil)

	store.On("GetByID", mock.Anything, "missing").
		Return((*interfaces.Category)(nil), errs.ErrNoSuchEntity)

	_, err := svc.Get(context.Background(), "missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNoSuchEntity))
}

// ---------------------------------------------------------------------------
// CategoryService.GetBySlug
// ---------------------------------------------------------------------------

func TestCategoryService_GetBySlug_Success(t *testing.T) {
	t.Parallel()
	store := new(MockCategoryStore)
	svc := NewCategoryService(store, nil)

	store.On("GetBySlug", mock.Anything, "chemises").
		Return(&interfaces.Category{Slug: "chemises", Name: "Chemises"}, nil)

	got, err := svc.GetBySlug(context.Background(), "chemises")
	require.NoError(t, err)
	assert.Equal(t, "Chemises", got.Name)
}

func TestCategoryService_GetBySlug_NotFound(t *testing.T) {
	t.Parallel()
	store := new(MockCategoryStore)
	svc := NewCategoryService(store, nil)

	store.On("GetBySlug", mock.Anything, "nope").
		Return((*interfaces.Category)(nil), errs.ErrNoSuchEntity)

	_, err := svc.GetBySlug(context.Background(), "nope")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNoSuchEntity))
}

// ---------------------------------------------------------------------------
// CategoryService.List / ListTree
// ---------------------------------------------------------------------------

func TestCategoryService_List_Success(t *testing.T) {
	t.Parallel()
	store := new(MockCategoryStore)
	svc := NewCategoryService(store, nil)

	store.On("List", mock.Anything).Return([]*interfaces.Category{
		{Name: "A"}, {Name: "B"},
	}, nil)

	got, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestCategoryService_ListTree_Success(t *testing.T) {
	t.Parallel()
	store := new(MockCategoryStore)
	svc := NewCategoryService(store, nil)

	store.On("ListTree", mock.Anything).Return([]*interfaces.Category{{Name: "Root"}}, nil)

	got, err := svc.ListTree(context.Background())
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

// ---------------------------------------------------------------------------
// CategoryService.Update
// ---------------------------------------------------------------------------

func TestCategoryService_Update_NameAndSlug(t *testing.T) {
	t.Parallel()
	store := new(MockCategoryStore)
	svc := NewCategoryService(store, nil)

	name := "Pulls"
	catID := uuid.New()
	store.On("Update", mock.Anything, catID.String(), mock.MatchedBy(func(f map[string]interface{}) bool {
		return f["name"] == "Pulls" && f["slug"] != nil
	})).Return(&interfaces.Category{ID: catID, Name: "Pulls"}, nil)

	got, err := svc.Update(context.Background(), catID.String(), &api.UpdateCategoryRequest{
		Name: &name,
	})
	require.NoError(t, err)
	assert.Equal(t, "Pulls", got.Name)
}

func TestCategoryService_Update_InvalidParentID(t *testing.T) {
	t.Parallel()
	store := new(MockCategoryStore)
	svc := NewCategoryService(store, nil)

	bad := "not-a-uuid"
	_, err := svc.Update(context.Background(), uuid.New().String(), &api.UpdateCategoryRequest{
		ParentID: &bad,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid parent_id")
}

func TestCategoryService_Update_ClearParentID(t *testing.T) {
	t.Parallel()
	store := new(MockCategoryStore)
	svc := NewCategoryService(store, nil)

	empty := ""
	catID := uuid.New()
	store.On("Update", mock.Anything, catID.String(), mock.MatchedBy(func(f map[string]interface{}) bool {
		return f["parent_id"] == nil
	})).Return(&interfaces.Category{ID: catID}, nil)

	_, err := svc.Update(context.Background(), catID.String(), &api.UpdateCategoryRequest{
		ParentID: &empty,
	})
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// CategoryService.Delete
// ---------------------------------------------------------------------------

func TestCategoryService_Delete_Success(t *testing.T) {
	t.Parallel()
	store := new(MockCategoryStore)
	svc := NewCategoryService(store, nil)

	store.On("Delete", mock.Anything, "id-1").Return(nil)

	err := svc.Delete(context.Background(), "id-1")
	require.NoError(t, err)
	store.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// CategoryService.SeedDefaults
// ---------------------------------------------------------------------------

func TestCategoryService_SeedDefaults_Success(t *testing.T) {
	t.Parallel()
	store := new(MockCategoryStore)
	svc := NewCategoryService(store, nil)

	store.On("SeedDefaultCategories", mock.Anything).Return(nil)

	err := svc.SeedDefaults(context.Background())
	require.NoError(t, err)
	store.AssertExpectations(t)
}
