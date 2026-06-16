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
// ProductService.Create
// ---------------------------------------------------------------------------

func TestProductService_Create_Success(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	prodID := uuid.New()
	store.On("Create", mock.Anything, mock.AnythingOfType("*interfaces.Product"),
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&interfaces.Product{ID: prodID, Name: "Chemise", Slug: "chemise"}, nil)

	got, err := svc.Create(context.Background(), &api.CreateProductRequest{
		Name:     "Chemise",
		Price:    89.99,
		IsActive: true,
	})
	require.NoError(t, err)
	assert.Equal(t, prodID, got.ID)
	store.AssertExpectations(t)
}

func TestProductService_Create_WithCategoryID(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	catID := uuid.New().String()
	store.On("Create", mock.Anything, mock.MatchedBy(func(p *interfaces.Product) bool {
		return p.CategoryID != nil
	}), mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&interfaces.Product{ID: uuid.New()}, nil)

	_, err := svc.Create(context.Background(), &api.CreateProductRequest{
		Name:       "T-shirt",
		Price:      49.0,
		CategoryID: &catID,
	})
	require.NoError(t, err)
}

func TestProductService_Create_InvalidCategoryID(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	bad := "not-uuid"
	_, err := svc.Create(context.Background(), &api.CreateProductRequest{
		Name:       "Bad",
		Price:      10,
		CategoryID: &bad,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid category_id")
}

func TestProductService_Create_WithVariants(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	store.On("Create", mock.Anything, mock.Anything,
		mock.MatchedBy(func(imgs []interfaces.ProductImage) bool { return len(imgs) == 1 }),
		mock.MatchedBy(func(sizes []interfaces.ProductSize) bool { return len(sizes) == 1 }),
		mock.MatchedBy(func(colors []interfaces.ProductColor) bool { return len(colors) == 1 }),
		mock.MatchedBy(func(variants []interfaces.ProductVariant) bool {
			return len(variants) == 1 && variants[0].Hex == "#FF0000"
		}),
	).Return(&interfaces.Product{ID: uuid.New()}, nil)

	_, err := svc.Create(context.Background(), &api.CreateProductRequest{
		Name:  "Full",
		Price: 100,
		Images: []api.CreateProductImageRequest{
			{URL: "https://img.com/1.jpg", IsPrimary: true},
		},
		Sizes:  []api.CreateProductSizeRequest{{Size: "M", Stock: 10}},
		Colors: []api.CreateProductColorRequest{{Name: "Rouge", Hex: "#FF0000", Stock: 10}},
		Variants: []api.CreateProductVariantRequest{
			{Size: "M", Color: "Rouge", Hex: "#FF0000", Stock: 10},
		},
	})
	require.NoError(t, err)
	store.AssertExpectations(t)
}

func TestProductService_Create_VariantDefaultHex(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	store.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.MatchedBy(func(variants []interfaces.ProductVariant) bool {
			return len(variants) == 1 && variants[0].Hex == "#000000"
		}),
	).Return(&interfaces.Product{ID: uuid.New()}, nil)

	_, err := svc.Create(context.Background(), &api.CreateProductRequest{
		Name:  "NoHex",
		Price: 50,
		Variants: []api.CreateProductVariantRequest{
			{Size: "L", Color: "Noir", Hex: "", Stock: 5},
		},
	})
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// ProductService.Get
// ---------------------------------------------------------------------------

func TestProductService_Get_Success(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	prodID := uuid.New()
	store.On("GetByID", mock.Anything, prodID.String()).
		Return(&interfaces.Product{ID: prodID, Name: "Prod"}, nil)
	store.On("IncrementViewCount", mock.Anything, prodID.String()).Return(nil)

	got, err := svc.Get(context.Background(), prodID.String())
	require.NoError(t, err)
	assert.Equal(t, "Prod", got.Name)
}

func TestProductService_Get_NotFound(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	store.On("GetByID", mock.Anything, "missing").
		Return((*interfaces.Product)(nil), errs.ErrNoSuchEntity)

	_, err := svc.Get(context.Background(), "missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNoSuchEntity))
}

// ---------------------------------------------------------------------------
// ProductService.GetBySlug
// ---------------------------------------------------------------------------

func TestProductService_GetBySlug_Success(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	prodID := uuid.New()
	store.On("GetBySlug", mock.Anything, "chemise-ete").
		Return(&interfaces.Product{ID: prodID, Slug: "chemise-ete"}, nil)
	store.On("IncrementViewCount", mock.Anything, prodID.String()).Return(nil)

	got, err := svc.GetBySlug(context.Background(), "chemise-ete")
	require.NoError(t, err)
	assert.Equal(t, "chemise-ete", got.Slug)
}

func TestProductService_GetBySlug_NotFound(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	store.On("GetBySlug", mock.Anything, "nope").
		Return((*interfaces.Product)(nil), errs.ErrNoSuchEntity)

	_, err := svc.GetBySlug(context.Background(), "nope")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNoSuchEntity))
}

// ---------------------------------------------------------------------------
// ProductService.GetByIDAdmin (no view increment)
// ---------------------------------------------------------------------------

func TestProductService_GetByIDAdmin_Success(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	prodID := uuid.New()
	store.On("GetByID", mock.Anything, prodID.String()).
		Return(&interfaces.Product{ID: prodID}, nil)

	got, err := svc.GetByIDAdmin(context.Background(), prodID.String())
	require.NoError(t, err)
	assert.Equal(t, prodID, got.ID)
	// IncrementViewCount should NOT be called.
	store.AssertNotCalled(t, "IncrementViewCount", mock.Anything, mock.Anything)
}

// ---------------------------------------------------------------------------
// ProductService.List
// ---------------------------------------------------------------------------

func TestProductService_List_Success(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	filters := &api.ProductFilters{Limit: 10, Offset: 0}
	store.On("List", mock.Anything, filters).
		Return([]*interfaces.Product{{Name: "A"}, {Name: "B"}}, int64(2), nil)

	got, err := svc.List(context.Background(), filters)
	require.NoError(t, err)
	assert.Len(t, got.Products, 2)
	assert.Equal(t, int64(2), got.TotalCount)
	assert.False(t, got.HasMore)
	assert.Equal(t, 10, got.Limit)
}

func TestProductService_List_DefaultLimit(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	filters := &api.ProductFilters{Limit: 0}
	store.On("List", mock.Anything, filters).
		Return([]*interfaces.Product{}, int64(0), nil)

	got, err := svc.List(context.Background(), filters)
	require.NoError(t, err)
	assert.Equal(t, 20, got.Limit) // default limit
}

func TestProductService_List_HasMore(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	filters := &api.ProductFilters{Limit: 2, Offset: 0}
	store.On("List", mock.Anything, filters).
		Return([]*interfaces.Product{{Name: "A"}, {Name: "B"}}, int64(5), nil)

	got, err := svc.List(context.Background(), filters)
	require.NoError(t, err)
	assert.True(t, got.HasMore)
}

// ---------------------------------------------------------------------------
// ProductService.GetSimilar
// ---------------------------------------------------------------------------

func TestProductService_GetSimilar_Success(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	prodID := uuid.New().String()
	store.On("GetSimilar", mock.Anything, prodID, 5).
		Return([]*interfaces.Product{{Name: "Sim1"}}, nil)

	got, err := svc.GetSimilar(context.Background(), prodID, 5)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

// ---------------------------------------------------------------------------
// ProductService.Update
// ---------------------------------------------------------------------------

func TestProductService_Update_Success(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	prodID := uuid.New()
	store.On("Update", mock.Anything, prodID.String(), mock.Anything).
		Return(&interfaces.Product{ID: prodID}, nil)
	store.On("UpsertImages", mock.Anything, prodID.String(), mock.Anything).Return(nil)
	store.On("UpsertSizes", mock.Anything, prodID.String(), mock.Anything).Return(nil)
	store.On("UpsertColors", mock.Anything, prodID.String(), mock.Anything).Return(nil)
	store.On("UpsertVariants", mock.Anything, prodID.String(), mock.Anything).Return(nil)
	store.On("GetByID", mock.Anything, prodID.String()).
		Return(&interfaces.Product{ID: prodID, Name: "Updated"}, nil)

	got, err := svc.Update(context.Background(), prodID.String(), &api.UpdateProductRequest{
		Name:  "Updated",
		Price: 99,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated", got.Name)
	store.AssertExpectations(t)
}

func TestProductService_Update_EmptyName(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	_, err := svc.Update(context.Background(), uuid.New().String(), &api.UpdateProductRequest{
		Name: "",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestProductService_Update_InvalidCategoryID(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	_, err := svc.Update(context.Background(), uuid.New().String(), &api.UpdateProductRequest{
		Name:       "Prod",
		CategoryID: "bad-uuid",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid category_id")
}

// ---------------------------------------------------------------------------
// ProductService.Delete
// ---------------------------------------------------------------------------

func TestProductService_Delete_Success(t *testing.T) {
	t.Parallel()
	store := new(MockProductStore)
	svc := NewProductService(store, nil)

	store.On("Delete", mock.Anything, "id-1").Return(nil)

	err := svc.Delete(context.Background(), "id-1")
	require.NoError(t, err)
	store.AssertExpectations(t)
}
