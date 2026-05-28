package handler

import (
	"errors"
	"fmt"
	"net/http"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/asaskevich/govalidator"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/middleware"
	"github.com/malekradhouane/magic/service"
	"github.com/malekradhouane/magic/utils/httpresp"
	"github.com/malekradhouane/magic/utils/phone"
)

// OrderHandler exposes order endpoints
type OrderHandler struct {
	service      *service.OrderService
	auth         gin.HandlerFunc
	optionalAuth gin.HandlerFunc
}

// NewOrderHandler constructs an OrderHandler
func NewOrderHandler(s *service.OrderService, auth, optionalAuth gin.HandlerFunc) *OrderHandler {
	return &OrderHandler{service: s, auth: auth, optionalAuth: optionalAuth}
}

// SetupRoutes registers order routes
// Public:
//
//	POST /orders                    (guest checkout supported)
//	GET  /orders/:id?phone=...       (guest lookup with phone validation)
//
// Auth:
//
//	GET  /me/orders                  (current user orders)
//
// Admin:
//
//	GET  /admin/orders
//	GET  /admin/orders/:id
//	PATCH /admin/orders/:id
func (h *OrderHandler) SetupRoutes(g *gin.RouterGroup) {
	// Public (guest-friendly); optionalAuth links orders to logged-in users when a Bearer token is sent.
	g.POST("/orders", h.optionalAuth, h.Create)
	g.GET("/orders/:id", h.optionalAuth, h.Get)

	// Authenticated (current user)
	authed := g.Group("")
	authed.Use(h.auth)
	authed.GET("/me/orders", h.ListMyOrders)

	// Admin — routes en chemins complets sous /api (évite les cas où Gin
	// ne matche pas correctement un sous-groupe /admin/orders avec GET "" + "/:id").
	adminAuthed := g.Group("")
	adminAuthed.Use(h.auth)
	adminAuthed.Use(middleware.RequireAdmin())
	adminAuthed.GET("/admin/orders", h.AdminList)
	adminAuthed.GET("/admin/orders/:id", h.AdminGet)
	adminAuthed.PATCH("/admin/orders/:id", h.UpdateStatus)
}

// Create creates a new order (guest or auth)
// @Summary Create order
// @Tags orders
// @Accept json
// @Produce json
// @Param order body api.CreateOrderRequest true "Order"
// @Success 201 {object} api.OrderResponse
// @Router /orders [post]
func (h *OrderHandler) Create(c *gin.Context) {
	var req api.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := govalidator.ValidateStruct(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	// Phone validation: the admin will call the customer to confirm the order,
	// so we must guarantee the number is a valid Tunisian phone. We also
	// normalize it ("+216XXXXXXXX") for consistent storage/lookup.
	normalizedPhone, err := phone.Normalize(req.ShippingInfo.Phone)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	req.ShippingInfo.Phone = normalizedPhone

	// Try to extract user ID from JWT (optional for guest checkout)
	userID := extractUserIDOptional(c)

	order, err := h.service.Create(c.Request.Context(), &req, userID)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	httpresp.NewResult(c, http.StatusCreated, &api.OrderResponse{
		Success: true,
		OrderID: order.ID.String(),
		Order:   order,
	})
}

// Get returns an order by ID
// For guest checkout, the phone query param is required to authorize access.
// @Summary Get order
// @Tags orders
// @Produce json
// @Param id path string true "Order ID"
// @Param phone query string false "Phone (required for guest orders)"
// @Success 200 {object} interfaces.Order
// @Router /orders/{id} [get]
func (h *OrderHandler) Get(c *gin.Context) {
	id := c.Param("id")
	phone := c.Query("phone")

	// If phone is provided, validate it matches (guest mode)
	if phone != "" {
		order, err := h.service.GetForCustomer(c.Request.Context(), id, phone)
		if err != nil {
			if errors.Is(err, errs.ErrNoSuchEntity) {
				httpresp.NewErrorMessage(c, http.StatusNotFound, "order not found")
				return
			}
			httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
			return
		}
		httpresp.NewResult(c, http.StatusOK, order)
		return
	}

	// Otherwise require auth
	userID := extractUserIDOptional(c)
	if userID == "" {
		httpresp.NewErrorMessage(c, http.StatusUnauthorized, "authentication or phone required")
		return
	}

	order, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, errs.ErrNoSuchEntity) {
			httpresp.NewErrorMessage(c, http.StatusNotFound, "order not found")
			return
		}
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Verify ownership
	if order.UserID == nil || order.UserID.String() != userID {
		httpresp.NewErrorMessage(c, http.StatusForbidden, "you do not own this order")
		return
	}

	httpresp.NewResult(c, http.StatusOK, order)
}

// ListMyOrders returns orders for the authenticated user
// @Summary List my orders
// @Tags orders
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} api.OrdersResponse
// @Router /me/orders [get]
func (h *OrderHandler) ListMyOrders(c *gin.Context) {
	userID := extractUserIDOptional(c)
	if userID == "" {
		httpresp.NewErrorMessage(c, http.StatusUnauthorized, "authentication required")
		return
	}
	if _, err := uuid.Parse(userID); err != nil {
		httpresp.NewErrorMessage(c, http.StatusUnauthorized, "invalid user identity")
		return
	}

	limit, _ := parseIntQuery(c, "limit", 20)
	offset, _ := parseIntQuery(c, "offset", 0)

	resp, err := h.service.ListByUser(c.Request.Context(), userID, limit, offset)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, resp)
}

// AdminList lists all orders (admin)
// @Summary List all orders (admin)
// @Tags orders
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} api.OrdersResponse
// @Router /admin/orders [get]
func (h *OrderHandler) AdminList(c *gin.Context) {
	filters := &api.ListOrdersFilters{}
	if err := c.ShouldBindQuery(filters); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.service.List(c.Request.Context(), filters)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, resp)
}

func (h *OrderHandler) AdminGet(c *gin.Context) {
	id := c.Param("id")
	order, err := h.service.GetForAdmin(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, errs.ErrNoSuchEntity) {
			httpresp.NewErrorMessage(c, http.StatusNotFound, "order not found")
			return
		}
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, order)
}

// UpdateStatus updates an order status (admin)
// @Summary Update order status
// @Tags orders
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Order ID"
// @Param order body api.UpdateOrderStatusRequest true "Status update"
// @Success 200 {object} interfaces.Order
// @Router /admin/orders/{id} [patch]
func (h *OrderHandler) UpdateStatus(c *gin.Context) {
	id := c.Param("id")
	var req api.UpdateOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	order, err := h.service.UpdateStatus(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, errs.ErrNoSuchEntity) {
			httpresp.NewErrorMessage(c, http.StatusNotFound, "order not found")
			return
		}
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, order)
}

// extractUserIDOptional reads the JWT user ID claim (returns "" if not authenticated)
func extractUserIDOptional(c *gin.Context) string {
	claims := jwt.ExtractClaims(c)
	if claims == nil {
		return ""
	}
	if id, ok := claims["id"].(string); ok {
		return id
	}
	return ""
}

// parseIntQuery parses an integer query param with a default fallback
func parseIntQuery(c *gin.Context, key string, def int) (int, error) {
	v := c.Query(key)
	if v == "" {
		return def, nil
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return def, err
	}
	return n, nil
}
