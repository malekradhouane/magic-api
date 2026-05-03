package handler

import (
	"errors"
	"net/http"

	"github.com/asaskevich/govalidator"
	"github.com/gin-gonic/gin"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/service"
	"github.com/malekradhouane/magic/utils/httpresp"
)

// CategoryHandler exposes category endpoints
type CategoryHandler struct {
	service *service.CategoryService
	auth    gin.HandlerFunc
}

// NewCategoryHandler constructs a CategoryHandler
func NewCategoryHandler(s *service.CategoryService, auth gin.HandlerFunc) *CategoryHandler {
	return &CategoryHandler{service: s, auth: auth}
}

// SetupRoutes registers category routes
// Public:
//
//	GET    /categories
//	GET    /categories/:slug
//
// Admin (auth):
//
//	POST   /categories
//	PATCH  /categories/:id
//	DELETE /categories/:id
func (h *CategoryHandler) SetupRoutes(g *gin.RouterGroup) {
	pub := g.Group("/categories")
	pub.GET("", h.List)
	pub.GET("/:slug", h.GetBySlug)

	admin := g.Group("/categories")
	admin.Use(h.auth)
	admin.POST("", h.Create)
	admin.PATCH("/:id", h.Update)
	admin.DELETE("/:id", h.Delete)
}

// List returns all active categories
// @Summary List categories
// @Tags categories
// @Produce json
// @Success 200 {array} interfaces.Category
// @Router /categories [get]
func (h *CategoryHandler) List(c *gin.Context) {
	cats, err := h.service.List(c.Request.Context())
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, cats)
}

// GetBySlug returns a category by its slug
// @Summary Get category by slug
// @Tags categories
// @Produce json
// @Param slug path string true "Category slug"
// @Success 200 {object} interfaces.Category
// @Failure 404 {object} httpresp.HTTPError
// @Router /categories/{slug} [get]
func (h *CategoryHandler) GetBySlug(c *gin.Context) {
	slug := c.Param("slug")
	cat, err := h.service.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		if errors.Is(err, errs.ErrNoSuchEntity) {
			httpresp.NewErrorMessage(c, http.StatusNotFound, "category not found")
			return
		}
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, cat)
}

// Create creates a new category (admin)
// @Summary Create category
// @Tags categories
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param category body api.CreateCategoryRequest true "Category"
// @Success 201 {object} interfaces.Category
// @Router /categories [post]
func (h *CategoryHandler) Create(c *gin.Context) {
	var req api.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := govalidator.ValidateStruct(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	cat, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusCreated, cat)
}

// Update updates a category (admin)
// @Summary Update category
// @Tags categories
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Category ID"
// @Param category body api.UpdateCategoryRequest true "Category fields"
// @Success 200 {object} interfaces.Category
// @Router /categories/{id} [patch]
func (h *CategoryHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req api.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	cat, err := h.service.Update(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, errs.ErrNoSuchEntity) {
			httpresp.NewErrorMessage(c, http.StatusNotFound, "category not found")
			return
		}
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, cat)
}

// Delete soft-deletes a category (admin)
// @Summary Delete category
// @Tags categories
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Category ID"
// @Success 204 "No Content"
// @Router /categories/{id} [delete]
func (h *CategoryHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
