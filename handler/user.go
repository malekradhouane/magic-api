package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/asaskevich/govalidator"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/magic/configmanager"
	"github.com/malekradhouane/magic/middleware"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/service"
	"github.com/malekradhouane/magic/utils/httpresp"
)

// UserHandler represents users handler actions
type UserHandler struct {
	userService *service.UserService
	authService *service.AuthService
	cman        configmanager.ManagerContract
	auth        gin.HandlerFunc
}

// NewUserHandler constructor
func NewUserHandler(us *service.UserService, authService *service.AuthService, auth gin.HandlerFunc, cman configmanager.ManagerContract) *UserHandler {
	return &UserHandler{
		userService: us,
		authService: authService,
		cman:        cman,
		auth:        auth,
	}
}

// SetupUsersRoutes creates routes for the provided group
func (uh *UserHandler) SetupUsersRoutes(g *gin.RouterGroup) *gin.RouterGroup {
	endpoint := "users"

	g.Group("/"+endpoint).POST("", uh.CreateUser)
	users := g.Group("/" + endpoint)
	{
		users.Use(uh.auth)
		users.GET("/:id", uh.GetUser)
		users.PATCH("/:id", uh.UpdateUser)
	}

	adminUsers := g.Group("/" + endpoint)
	{
		adminUsers.Use(uh.auth)
		adminUsers.Use(middleware.RequireAdmin())
		adminUsers.GET("", uh.GetUsers)
		adminUsers.DELETE("/:id", uh.DeleteUser)
	}

	return users
}

// CreateUser godoc
// @Summary Create a new user
// @Description Create a new user with the provided information
// @Tags users
// @Accept json
// @Produce json
// @Param user body api.SignUpRequest true "User information"
// @Success 201 {object} interfaces.User "User created successfully"
// @Failure 400 {object} gin.H "Data validation error"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 409 {object} gin.H "Email already in use"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /users [post]
func (uh *UserHandler) CreateUser(c *gin.Context) {
	req := new(api.SignUpRequest)

	err := c.ShouldBindBodyWith(req, binding.JSON)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	_, err = govalidator.ValidateStruct(req)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	// Use the unified auth service
	result, err := uh.authService.SignUpWithPassword(c.Request.Context(), req)
	if errors.Is(err, errs.ErrEmailTaken) {
		httpresp.NewErrorMessage(c, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Return the user without sensitive data
	response := map[string]interface{}{
		"user":        result.User,
		"is_new_user": result.IsNewUser,
	}

	httpresp.NewResult(c, http.StatusCreated, response)
}

// GetUsers retrieves a list of users and sends the result as an HTTP response with a status code of 200 OK.
// If an error occurs while fetching the users, it sends an HTTP 500 Internal Server Error with the error message.
// @Summary Get all users
// @Description Get all users
// @Tags users
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {array} interfaces.User "List of users"
// @Failure 500 {object} httpresp.HTTPError
// @Router /users [get]
func (uh *UserHandler) GetUsers(c *gin.Context) {
	var users []*interfaces.User
	users, err := uh.userService.GetUsers(c.Request.Context())
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, users)
}

// GetUser retrieves a user by ID and sends the result as an HTTP response with a status code of 200 OK.
// @Summary Get a user by ID
// @Description Get a user by ID
// @Tags users
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param id path string true "User ID"
// @Success 200 {object} interfaces.User "User"
// @Failure 500 {object} httpresp.HTTPError
// @Router /users/{id} [get]
func (uh *UserHandler) GetUser(c *gin.Context) {
	id := c.Param("id")
	user, err := uh.userService.GetUser(c.Request.Context(), id)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, user)
}

// UpdateUser handles HTTP PATCH /users/:id
// @Summary Update a user
// @Description Update user information
// @Tags users
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param id path string true "User ID"
// @Param input body api.UpdateUserRequest true "User update data"
// @Success 200 {object} interfaces.User
// @Failure 400 {object} httpresp.HTTPError
// @Failure 404 {object} httpresp.HTTPError
// @Failure 500 {object} httpresp.HTTPError
// @Router /users/{id} [patch]
func (uh *UserHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpresp.NewError(c, http.StatusBadRequest, fmt.Errorf("user ID is required"))
		return
	}

	var req api.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewError(c, http.StatusBadRequest, fmt.Errorf("invalid request body: %v", err))
		return
	}

	updatedUser, err := uh.userService.UpdateUser(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, errs.ErrNoSuchEntity) {
			httpresp.NewError(c, http.StatusNotFound, fmt.Errorf("user not found"))
			return
		}
		httpresp.NewError(c, http.StatusInternalServerError, fmt.Errorf("failed to update user: %v", err))
		return
	}

	httpresp.NewResult(c, http.StatusOK, updatedUser)
}

// DeleteUser handles HTTP DELETE /users/:id
// @Summary Delete a user
// @Description Delete a user by ID
// @Tags users
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param id path string true "User ID"
// @Success 204 "No Content"
// @Failure 400 {object} httpresp.HTTPError
// @Failure 404 {object} httpresp.HTTPError
// @Failure 500 {object} httpresp.HTTPError
// @Router /users/{id} [delete]
func (uh *UserHandler) DeleteUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpresp.NewError(c, http.StatusBadRequest, fmt.Errorf("user ID is required"))
		return
	}

	err := uh.userService.DeleteUser(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, errs.ErrNoSuchEntity) {
			httpresp.NewError(c, http.StatusNotFound, fmt.Errorf("user not found"))
			return
		}
		httpresp.NewError(c, http.StatusInternalServerError, fmt.Errorf("failed to delete user: %v", err))
		return
	}

	c.Status(http.StatusNoContent)
}
