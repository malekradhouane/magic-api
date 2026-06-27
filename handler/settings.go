package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/middleware"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/service"
	"github.com/malekradhouane/magic/utils/httpresp"
)

// SettingsHandler exposes admin settings endpoints.
type SettingsHandler struct {
	service *service.SettingsService
	auth    gin.HandlerFunc
}

// NewSettingsHandler constructs a SettingsHandler
func NewSettingsHandler(s *service.SettingsService, auth gin.HandlerFunc) *SettingsHandler {
	return &SettingsHandler{service: s, auth: auth}
}

// SetupRoutes registers settings routes
// Admin (auth):
//
//	GET /admin/settings          — list all settings groups
//	GET /admin/settings/:key     — get one settings group
//	PUT /admin/settings/:key     — update one settings group
func (h *SettingsHandler) SetupRoutes(g *gin.RouterGroup) {
	admin := g.Group("/admin/settings")
	admin.Use(h.auth)
	admin.Use(middleware.RequireAdmin())
	admin.GET("", h.GetAll)
	admin.GET("/:key", h.GetByKey)
	admin.PUT("/:key", h.Update)
}

// GetAll returns every settings group.
// @Summary List all settings (admin)
// @Tags settings
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {array} interfaces.Setting
// @Router /admin/settings [get]
func (h *SettingsHandler) GetAll(c *gin.Context) {
	settings, err := h.service.GetAll(c.Request.Context())
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, settings)
}

// GetByKey returns a single settings group.
// @Summary Get a settings group (admin)
// @Tags settings
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param key path string true "Settings key (general, shipping, notifications, seo)"
// @Success 200 {object} interfaces.Setting
// @Failure 404 {object} httpresp.HTTPError
// @Router /admin/settings/{key} [get]
func (h *SettingsHandler) GetByKey(c *gin.Context) {
	key := c.Param("key")
	setting, err := h.service.GetByKey(c.Request.Context(), key)
	if err != nil {
		if errors.Is(err, errs.ErrNoSuchEntity) {
			httpresp.NewErrorMessage(c, http.StatusNotFound, "setting not found")
			return
		}
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, setting)
}

// Update upserts a settings group.
// @Summary Update a settings group (admin)
// @Tags settings
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param key path string true "Settings key"
// @Param setting body api.UpdateSettingRequest true "Setting value"
// @Success 200 {object} interfaces.Setting
// @Router /admin/settings/{key} [put]
func (h *SettingsHandler) Update(c *gin.Context) {
	key := c.Param("key")
	var req api.UpdateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	setting, err := h.service.Update(c.Request.Context(), key, interfaces.SettingsValue(req.Value))
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, setting)
}
