package handler

import (
	"errors"
	"net/http"

	"github.com/asaskevich/govalidator"
	"github.com/gin-gonic/gin"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/service"
	"github.com/malekradhouane/magic/utils/httpresp"
)

// AddressHandler exposes user address endpoints (scoped to authenticated user).
type AddressHandler struct {
	service *service.AddressService
	auth    gin.HandlerFunc
}

// NewAddressHandler constructs an AddressHandler.
func NewAddressHandler(s *service.AddressService, auth gin.HandlerFunc) *AddressHandler {
	return &AddressHandler{service: s, auth: auth}
}

// SetupRoutes registers address routes under /me/addresses (auth required).
func (h *AddressHandler) SetupRoutes(g *gin.RouterGroup) {
	me := g.Group("/me")
	me.Use(h.auth)
	{
		me.GET("/addresses", h.List)
		me.POST("/addresses", h.Create)
		me.PATCH("/addresses/:id", h.Update)
		me.DELETE("/addresses/:id", h.Delete)
		me.PATCH("/addresses/:id/default", h.SetDefault)
	}
}

// List returns addresses for the authenticated user only.
func (h *AddressHandler) List(c *gin.Context) {
	userID := extractUserIDOptional(c)
	if userID == "" {
		httpresp.NewErrorMessage(c, http.StatusUnauthorized, "authentication required")
		return
	}

	addresses, err := h.service.ListByUser(c.Request.Context(), userID)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	if addresses == nil {
		addresses = []*interfaces.Address{}
	}
	httpresp.NewResult(c, http.StatusOK, &api.AddressesResponse{Addresses: addresses})
}

// Create adds a new address for the authenticated user.
func (h *AddressHandler) Create(c *gin.Context) {
	userID := extractUserIDOptional(c)
	if userID == "" {
		httpresp.NewErrorMessage(c, http.StatusUnauthorized, "authentication required")
		return
	}

	var req api.CreateAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := govalidator.ValidateStruct(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	addr, err := h.service.Create(c.Request.Context(), userID, &req)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusCreated, addr)
}

// Update modifies an address owned by the authenticated user.
func (h *AddressHandler) Update(c *gin.Context) {
	userID := extractUserIDOptional(c)
	if userID == "" {
		httpresp.NewErrorMessage(c, http.StatusUnauthorized, "authentication required")
		return
	}

	var req api.UpdateAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	addr, err := h.service.Update(c.Request.Context(), userID, c.Param("id"), &req)
	if err != nil {
		if errors.Is(err, errs.ErrNoSuchEntity) {
			httpresp.NewErrorMessage(c, http.StatusNotFound, "address not found")
			return
		}
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, addr)
}

// Delete removes an address owned by the authenticated user.
func (h *AddressHandler) Delete(c *gin.Context) {
	userID := extractUserIDOptional(c)
	if userID == "" {
		httpresp.NewErrorMessage(c, http.StatusUnauthorized, "authentication required")
		return
	}

	if err := h.service.Delete(c.Request.Context(), userID, c.Param("id")); err != nil {
		if errors.Is(err, errs.ErrNoSuchEntity) {
			httpresp.NewErrorMessage(c, http.StatusNotFound, "address not found")
			return
		}
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// SetDefault marks an address as the user's default.
func (h *AddressHandler) SetDefault(c *gin.Context) {
	userID := extractUserIDOptional(c)
	if userID == "" {
		httpresp.NewErrorMessage(c, http.StatusUnauthorized, "authentication required")
		return
	}

	if err := h.service.SetDefault(c.Request.Context(), userID, c.Param("id")); err != nil {
		if errors.Is(err, errs.ErrNoSuchEntity) {
			httpresp.NewErrorMessage(c, http.StatusNotFound, "address not found")
			return
		}
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
