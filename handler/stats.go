package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/malekradhouane/magic/middleware"
	"github.com/malekradhouane/magic/service"
	"github.com/malekradhouane/magic/utils/httpresp"
)

// StatsHandler exposes the admin dashboard statistics endpoints.
type StatsHandler struct {
	service *service.StatsService
	auth    gin.HandlerFunc
}

// NewStatsHandler constructs a StatsHandler
func NewStatsHandler(s *service.StatsService, auth gin.HandlerFunc) *StatsHandler {
	return &StatsHandler{service: s, auth: auth}
}

// SetupRoutes registers stats routes
// Admin (auth):
//
//	GET /admin/stats/overview?days=30
func (h *StatsHandler) SetupRoutes(g *gin.RouterGroup) {
	admin := g.Group("/admin/stats")
	admin.Use(h.auth)
	admin.Use(middleware.RequireAdmin())
	admin.GET("/overview", h.Overview)
}

// Overview returns the full dashboard payload.
// @Summary Dashboard overview (admin)
// @Tags stats
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param days query int false "Lookback window in days (default 30, max 365)"
// @Success 200 {object} api.StatsOverview
// @Router /admin/stats/overview [get]
func (h *StatsHandler) Overview(c *gin.Context) {
	days := 30
	if v := c.Query("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}

	overview, err := h.service.Overview(c.Request.Context(), days)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, overview)
}
