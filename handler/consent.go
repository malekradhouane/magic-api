package handler

import (
	"net/http"
	"strconv"

	"github.com/asaskevich/govalidator"
	"github.com/gin-gonic/gin"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/middleware"
	"github.com/malekradhouane/magic/service"
	"github.com/malekradhouane/magic/utils/httpresp"
)

// ConsentHandler exposes public contact/newsletter endpoints and an admin
// endpoint to retrieve stored consent proofs.
type ConsentHandler struct {
	service  *service.ConsentService
	auth     gin.HandlerFunc
	submitRL gin.HandlerFunc
}

// NewConsentHandler constructs a ConsentHandler. submitRL rate-limits the public
// submission endpoints to prevent spam; pass nil to disable (e.g. in tests).
func NewConsentHandler(s *service.ConsentService, auth, submitRL gin.HandlerFunc) *ConsentHandler {
	return &ConsentHandler{service: s, auth: auth, submitRL: submitRL}
}

// SetupRoutes registers consent routes.
// Public:
//
//	POST /contact
//	POST /newsletter/subscribe
//
// Admin:
//
//	GET /admin/consents
func (h *ConsentHandler) SetupRoutes(g *gin.RouterGroup) {
	if h.submitRL != nil {
		g.POST("/contact", h.submitRL, h.SubmitContact)
		g.POST("/newsletter/subscribe", h.submitRL, h.SubscribeNewsletter)
	} else {
		g.POST("/contact", h.SubmitContact)
		g.POST("/newsletter/subscribe", h.SubscribeNewsletter)
	}

	admin := g.Group("/admin/consents")
	admin.Use(h.auth)
	admin.Use(middleware.RequireAdmin())
	admin.GET("", h.List)
}

// SubmitContact stores a contact-form submission with its consent proof.
// @Summary Submit contact form
// @Tags consent
// @Accept json
// @Produce json
// @Param body body api.ContactRequest true "Contact payload"
// @Success 201 {object} api.ConsentResponse
// @Router /contact [post]
func (h *ConsentHandler) SubmitContact(c *gin.Context) {
	var req api.ContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := govalidator.ValidateStruct(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	if !req.Consent {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, "consent is required")
		return
	}

	stored, err := h.service.SubmitContact(c.Request.Context(), &req, consentMeta(c))
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusCreated, &api.ConsentResponse{
		ID:      stored.ID.String(),
		Message: "Message envoyé avec succès",
	})
}

// SubscribeNewsletter stores a newsletter subscription with its consent proof.
// @Summary Subscribe to newsletter
// @Tags consent
// @Accept json
// @Produce json
// @Param body body api.NewsletterSubscribeRequest true "Newsletter payload"
// @Success 201 {object} api.ConsentResponse
// @Router /newsletter/subscribe [post]
func (h *ConsentHandler) SubscribeNewsletter(c *gin.Context) {
	var req api.NewsletterSubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := govalidator.ValidateStruct(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	if !req.Consent {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, "consent is required")
		return
	}

	stored, err := h.service.SubscribeNewsletter(c.Request.Context(), &req, consentMeta(c))
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusCreated, &api.ConsentResponse{
		ID:      stored.ID.String(),
		Message: "Inscription prise en compte",
	})
}

// List returns stored consent proofs (admin only).
// @Summary List consent proofs
// @Tags consent
// @Produce json
// @Param type query string false "Filter by type (contact|newsletter)"
// @Param limit query int false "Page size"
// @Param offset query int false "Offset"
// @Success 200 {object} api.ConsentsResponse
// @Router /admin/consents [get]
func (h *ConsentHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	consents, total, err := h.service.List(c.Request.Context(), c.Query("type"), limit, offset)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, &api.ConsentsResponse{Consents: consents, Total: total})
}

// consentMeta captures request-scoped metadata for consent proof.
func consentMeta(c *gin.Context) service.ConsentMeta {
	return service.ConsentMeta{
		IPAddress: c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}
}
