package httpresp

import (
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/errs"
)

// NewErrorMessage writes a JSON error response. For 5xx responses the
// real error is logged server-side and a generic message is returned so
// internal details (stack traces, query fragments, …) do not leak.
//
// By default the masking is ALWAYS applied; set DEBUG_HTTP_ERRORS=true to
// surface the raw 5xx message in development. Previously the masking only
// activated when GIN_MODE=release, which silently leaked errors in staging.
func NewErrorMessage(ctx *gin.Context, status int, err string) {
	if status >= 500 && !debugHTTPErrors() {
		logrus.WithField("path", ctx.Request.URL.Path).Error(err)
		NewError(ctx, status, errors.New("internal server error"))
		return
	}
	NewError(ctx, status, errors.New(err))
}

func debugHTTPErrors() bool {
	v := os.Getenv("DEBUG_HTTP_ERRORS")
	return v == "1" || v == "true" || v == "TRUE"
}

// NewError sets error to context
func NewError(ctx *gin.Context, status int, err error) {
	er := HTTPError{
		Success: false,
		Code:    status,
		Error:   err.Error(),
	}
	ctx.JSON(status, er)
}

// MapError maps a domain/service error to an HTTP status and message.
func MapError(err error) (int, string) {
	switch {
	case errors.Is(err, errs.ErrNoSuchEntity):
		return http.StatusNotFound, errs.ErrNoSuchEntity.Error()
	case errors.Is(err, errs.ErrEmailTaken):
		return http.StatusConflict, errs.ErrEmailTaken.Error()
	case errors.Is(err, errs.ErrUserNil),
		errors.Is(err, errs.ErrUserIDRequired),
		errors.Is(err, errs.ErrUserIDMissing),
		errors.Is(err, errs.ErrCompanyIDRequired),
		errors.Is(err, errs.ErrOrgIDRequired),
		errors.Is(err, errs.ErrEmptyUpdate):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, errs.ErrCompanyNotFound),
		errors.Is(err, errs.ErrOrganizationNotFound):
		return http.StatusNotFound, err.Error()
	default:
		return http.StatusInternalServerError, "internal server error"
	}
}

// FromError writes a standardized HTTP error response based on a domain/service error.
func FromError(ctx *gin.Context, err error) {
	if err == nil {
		return
	}
	status, msg := MapError(err)
	NewErrorMessage(ctx, status, msg)
}

// HTTPError is the JSON shape returned for every error.
type HTTPError struct {
	Success bool   `json:"success" example:"false"`
	Code    int    `json:"code" example:"500"`
	Error   string `json:"error" example:"internal server error"`
}

// HTTPError400 example.
type HTTPError400 struct {
	Success bool   `json:"success" example:"false"`
	Code    int    `json:"code" example:"400"`
	Error   string `json:"error" example:"bad request"`
}

// HTTPError401 example.
type HTTPError401 struct {
	Success bool   `json:"success" example:"false"`
	Code    int    `json:"code" example:"401"`
	Error   string `json:"error" example:"unauthorized"`
}

// HTTPError403 example.
type HTTPError403 struct {
	Success bool   `json:"success" example:"false"`
	Code    int    `json:"code" example:"403"`
	Error   string `json:"error" example:"forbidden"`
}

// HTTPError404 example.
type HTTPError404 struct {
	Success bool   `json:"success" example:"false"`
	Code    int    `json:"code" example:"404"`
	Error   string `json:"error" example:"not found"`
}

// HTTPError409 example.
type HTTPError409 struct {
	Success bool   `json:"success" example:"false"`
	Code    int    `json:"code" example:"409"`
	Error   string `json:"error" example:"conflict"`
}

// HTTPError500 example.
type HTTPError500 struct {
	Success bool   `json:"success" example:"false"`
	Code    int    `json:"code" example:"500"`
	Error   string `json:"error" example:"internal server error"`
}

// HTTPError501 example.
type HTTPError501 struct {
	Success bool   `json:"success" example:"false"`
	Code    int    `json:"code" example:"501"`
	Error   string `json:"error" example:"not Implemented"`
}

// HTTPError503 example.
type HTTPError503 struct {
	Success bool   `json:"success" example:"false"`
	Code    int    `json:"code" example:"503"`
	Error   string `json:"error" example:"service unavailable"`
}
