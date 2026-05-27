package handler

import (
	"net/http"

	"github.com/asaskevich/govalidator"
	"github.com/gin-gonic/gin"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/middleware"
	"github.com/malekradhouane/magic/service"
	"github.com/malekradhouane/magic/utils/httpresp"
)

// PromoHandler exposes promo-code endpoints
type PromoHandler struct {
	service *service.PromoService
	auth    gin.HandlerFunc
}

// NewPromoHandler constructs a PromoHandler
func NewPromoHandler(s *service.PromoService, auth gin.HandlerFunc) *PromoHandler {
	return &PromoHandler{service: s, auth: auth}
}

// SetupRoutes registers promo routes
// Public:
//
//	POST /promo/validate
//
// Admin:
//
//	POST /admin/promos
//	GET  /admin/promos
//	DELETE /admin/promos/:id
func (h *PromoHandler) SetupRoutes(g *gin.RouterGroup) {
	g.POST("/promo/validate", h.Validate)

	admin := g.Group("/admin/promos")
	admin.Use(h.auth)
	admin.Use(middleware.RequireAdmin())
	admin.POST("", h.Create)
	admin.GET("", h.List)
	admin.DELETE("/:id", h.Delete)
}

// Validate checks a promo code
// @Summary Validate promo code
// @Tags promo
// @Accept json
// @Produce json
// @Param body body api.ValidatePromoRequest true "Code + subtotal"
// @Success 200 {object} api.ValidatePromoResponse
// @Router /promo/validate [post]
func (h *PromoHandler) Validate(c *gin.Context) {
	var req api.ValidatePromoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := govalidator.ValidateStruct(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := extractUserIDOptional(c)

	resp, err := h.service.Validate(c.Request.Context(), req.Code, req.Subtotal, userID)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, resp)
}

// Create creates a promo code (admin)
// @Summary Create promo code
// @Tags promo
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param promo body api.CreatePromoRequest true "Promo"
// @Success 201 {object} interfaces.PromoCode
// @Router /admin/promos [post]
func (h *PromoHandler) Create(c *gin.Context) {
	var req api.CreatePromoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := govalidator.ValidateStruct(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	promo, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusCreated, promo)
}

// List returns all promo codes (admin)
// @Summary List promo codes (admin)
// @Tags promo
// @Param Authorization header string true "Bearer token"
// @Success 200 {array} interfaces.PromoCode
// @Router /admin/promos [get]
func (h *PromoHandler) List(c *gin.Context) {
	promos, err := h.service.List(c.Request.Context())
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, promos)
}

// Delete removes a promo code (admin)
// @Summary Delete promo code (admin)
// @Tags promo
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Promo ID"
// @Success 204 "No Content"
// @Router /admin/promos/{id} [delete]
func (h *PromoHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
