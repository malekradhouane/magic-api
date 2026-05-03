package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/asaskevich/govalidator"
	"github.com/gin-gonic/gin"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/service"
	"github.com/malekradhouane/magic/utils/httpresp"
)

// ProductHandler exposes product endpoints
type ProductHandler struct {
	service *service.ProductService
	auth    gin.HandlerFunc
}

// NewProductHandler constructs a ProductHandler
func NewProductHandler(s *service.ProductService, auth gin.HandlerFunc) *ProductHandler {
	return &ProductHandler{service: s, auth: auth}
}

// SetupRoutes registers product routes
// Public:
//
//	GET    /products
//	GET    /products/:slug
//	GET    /products/:slug/similar
//
// Admin:
//
//	POST   /products
//	PATCH  /products/:id
//	DELETE /products/:id
func (h *ProductHandler) SetupRoutes(g *gin.RouterGroup) {
	pub := g.Group("/products")
	pub.GET("", h.List)
	pub.GET("/:slug", h.GetBySlug)
	pub.GET("/:slug/similar", h.GetSimilar)

	admin := g.Group("/products")
	admin.Use(h.auth)
	admin.POST("", h.Create)
	admin.PATCH("/:id", h.Update)
	admin.DELETE("/:id", h.Delete)
}

// List returns paginated, filtered products
// @Summary List products
// @Tags products
// @Produce json
// @Param category query string false "Category slug or 'new' / 'soldes'"
// @Param sizes query []string false "Sizes filter"
// @Param colors query []string false "Colors filter"
// @Param min_price query number false "Min price"
// @Param max_price query number false "Max price"
// @Param sort query string false "Sort: relevance|newest|price-asc|price-desc"
// @Param search query string false "Search query"
// @Param limit query int false "Limit (default 20)"
// @Param offset query int false "Offset"
// @Success 200 {object} api.ProductsResponse
// @Router /products [get]
func (h *ProductHandler) List(c *gin.Context) {
	filters := &api.ProductFilters{}
	if err := c.ShouldBindQuery(filters); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	if filters.Limit <= 0 {
		filters.Limit = 20
	}

	resp, err := h.service.List(c.Request.Context(), filters)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, resp)
}

// GetBySlug returns a product detail by slug
// @Summary Get product by slug
// @Tags products
// @Produce json
// @Param slug path string true "Product slug"
// @Success 200 {object} interfaces.Product
// @Failure 404 {object} httpresp.HTTPError
// @Router /products/{slug} [get]
func (h *ProductHandler) GetBySlug(c *gin.Context) {
	slug := c.Param("slug")
	product, err := h.service.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		if errors.Is(err, errs.ErrNoSuchEntity) {
			httpresp.NewErrorMessage(c, http.StatusNotFound, "product not found")
			return
		}
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, product)
}

// GetSimilar returns similar products
// @Summary Get similar products
// @Tags products
// @Produce json
// @Param slug path string true "Product slug"
// @Param limit query int false "Limit (default 4)"
// @Success 200 {array} interfaces.Product
// @Router /products/{slug}/similar [get]
func (h *ProductHandler) GetSimilar(c *gin.Context) {
	slug := c.Param("slug")
	product, err := h.service.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		if errors.Is(err, errs.ErrNoSuchEntity) {
			httpresp.NewErrorMessage(c, http.StatusNotFound, "product not found")
			return
		}
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "4"))
	similar, err := h.service.GetSimilar(c.Request.Context(), product.ID.String(), limit)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, similar)
}

// Create creates a product (admin)
// @Summary Create product
// @Tags products
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param product body api.CreateProductRequest true "Product"
// @Success 201 {object} interfaces.Product
// @Router /products [post]
func (h *ProductHandler) Create(c *gin.Context) {
	var req api.CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := govalidator.ValidateStruct(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	product, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusCreated, product)
}

// Update updates product fields (admin)
// @Summary Update product
// @Tags products
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Product ID"
// @Param product body api.UpdateProductRequest true "Product fields"
// @Success 200 {object} interfaces.Product
// @Router /products/{id} [patch]
func (h *ProductHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req api.UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	product, err := h.service.Update(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, errs.ErrNoSuchEntity) {
			httpresp.NewErrorMessage(c, http.StatusNotFound, "product not found")
			return
		}
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, product)
}

// Delete soft-deletes a product (admin)
// @Summary Delete product
// @Tags products
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Product ID"
// @Success 204 "No Content"
// @Router /products/{id} [delete]
func (h *ProductHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
