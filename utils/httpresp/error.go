package httpresp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/malekradhouane/magic/errs"
)

// NewErrorMessage sets error message to context
func NewErrorMessage(ctx *gin.Context, status int, err string) {
	NewError(ctx, status, errors.New(err))
}

// // NewErrorMessage2 sets error message to context
// func NewErrorMessage2(ctx *gin.Context, eventCode string, status int, err string) {
// 	NewError2(ctx, eventCode, status, errors.New(err))
// }

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

// // NewError2 sets error to context
// func NewError2(ctx *gin.Context, eventCode string, status int, err error) {
// 	er := HTTPErrorTM{
// 		EventCode: eventCode,
// 		Success:   false,
// 		Code:      status,
// 		Error:     err.Error(),
// 	}
// 	ctx.JSON(status, er)
// }

// HTTPError example
type HTTPError struct {
	Success bool   `json:"success" example:"false"`               // wether the business action is successfull or not
	Code    int    `json:"code" example:"500"`                    // the HTTP status
	Error   string `json:"error" example:"internal server error"` // the error description
}

// HTTPError400 example
type HTTPError400 struct {
	Success bool   `json:"success" example:"false"`     // wether the business action is successfull or not
	Code    int    `json:"code" example:"400"`          // the HTTP status
	Error   string `json:"error" example:"bad request"` // the error description
}

// HTTPError401 example
type HTTPError401 struct {
	Success bool   `json:"success" example:"false"`      // wether the business action is successfull or not
	Code    int    `json:"code" example:"401"`           // the HTTP status
	Error   string `json:"error" example:"unauthorized"` // the error description
}

// HTTPError403 example
type HTTPError403 struct {
	Success bool   `json:"success" example:"false"`   // wether the business action is successfull or not
	Code    int    `json:"code" example:"403"`        // the HTTP status
	Error   string `json:"error" example:"forbidden"` // the error description
}

// HTTPError404 example
type HTTPError404 struct {
	Success bool   `json:"success" example:"false"`   // wether the business action is successfull or not
	Code    int    `json:"code" example:"404"`        // the HTTP status
	Error   string `json:"error" example:"not found"` // the error description
}

// HTTPError409 example
type HTTPError409 struct {
	Success bool   `json:"success" example:"false"`   // wether the business action is successfull or not
	Code    int    `json:"code" example:"409"`        // the HTTP status
	Error   string `json:"error" example:"forbidden"` // the error description
}

// HTTPError500 example
type HTTPError500 struct {
	Success bool   `json:"success" example:"false"`               // wether the business action is successfull or not
	Code    int    `json:"code" example:"500"`                    // the HTTP status
	Error   string `json:"error" example:"internal server error"` // the error description
}

// HTTPError501 example
type HTTPError501 struct {
	Success bool   `json:"success" example:"false"`         // wether the business action is successfull or not
	Code    int    `json:"code" example:"501"`              // the HTTP status
	Error   string `json:"error" example:"not Implemented"` // the error description
}

// HTTPError503 example
type HTTPError503 struct {
	Success bool   `json:"success" example:"false"`             // wether the business action is successfull or not
	Code    int    `json:"code" example:"503"`                  // the HTTP status
	Error   string `json:"error" example:"service unavailable"` // the error description
}
