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
	userService     *service.UserService
	authService     *service.AuthService
	cman            configmanager.ManagerContract
	auth            gin.HandlerFunc
	registerLimiter gin.HandlerFunc
}

// NewUserHandler constructor. registerRL is applied to the public POST /users
// endpoint to defeat bulk account creation and email enumeration. It may be
// the no-op handler in tests but should never be nil in production.
func NewUserHandler(
	us *service.UserService,
	authService *service.AuthService,
	auth gin.HandlerFunc,
	registerRL gin.HandlerFunc,
	cman configmanager.ManagerContract,
) *UserHandler {
	if registerRL == nil {
		registerRL = func(c *gin.Context) { c.Next() }
	}
	return &UserHandler{
		userService:     us,
		authService:     authService,
		cman:            cman,
		auth:            auth,
		registerLimiter: registerRL,
	}
}

// SetupUsersRoutes creates routes for the provided group
func (uh *UserHandler) SetupUsersRoutes(g *gin.RouterGroup) *gin.RouterGroup {
	endpoint := "users"

	// Public registration — rate-limited per IP to block account farming and
	// bcrypt-based DoS.
	g.Group("/"+endpoint).POST("", uh.registerLimiter, uh.CreateUser)

	// Self-service routes: a user may only read / update themselves unless
	// they have an admin-level role. Privileged fields cannot be set here.
	users := g.Group("/" + endpoint)
	{
		users.Use(uh.auth)
		users.GET("/:id", middleware.RequireSelfOrAdmin("id"), uh.GetUser)
		users.PATCH("/:id", middleware.RequireSelfOrAdmin("id"), uh.UpdateUser)
	}

	adminUsers := g.Group("/" + endpoint)
	{
		adminUsers.Use(uh.auth)
		adminUsers.Use(middleware.RequireAdmin())
		adminUsers.GET("", uh.GetUsers)
		adminUsers.GET("/admins", uh.GetAdmins)
		adminUsers.GET("/customers", uh.GetCustomers)
		// Admin-only privileged update (role, is_active, is_superuser, …).
		adminUsers.PATCH("/:id/admin", uh.AdminUpdateUser)
		adminUsers.DELETE("/:id", uh.DeleteUser)
	}

	// Superadmin-only routes for creating admins
	superAdminUsers := g.Group("/" + endpoint)
	{
		superAdminUsers.Use(uh.auth)
		superAdminUsers.Use(middleware.RequireAdmin())
		superAdminUsers.POST("/admins", uh.CreateAdmin)
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

	result, err := uh.authService.SignUpWithPassword(c.Request.Context(), req)
	if err != nil {
		httpresp.FromError(c, err)
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

// UpdateUser handles HTTP PATCH /users/:id for the authenticated user.
// Only safe profile fields are accepted; privileged fields such as is_active
// or is_superuser are rejected here even if the caller submits them. Admins
// must use PATCH /users/:id/admin to flip privileged fields.
// @Summary Update a user (self)
// @Description Update profile fields of the authenticated user
// @Tags users
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param id path string true "User ID"
// @Param input body api.UpdateUserSelfRequest true "User update data"
// @Success 200 {object} interfaces.User
// @Failure 400 {object} httpresp.HTTPError
// @Failure 403 {object} httpresp.HTTPError
// @Failure 404 {object} httpresp.HTTPError
// @Failure 500 {object} httpresp.HTTPError
// @Router /users/{id} [patch]
func (uh *UserHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpresp.NewError(c, http.StatusBadRequest, fmt.Errorf("user ID is required"))
		return
	}

	var req api.UpdateUserSelfRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewError(c, http.StatusBadRequest, fmt.Errorf("invalid request body: %v", err))
		return
	}

	updatedUser, err := uh.userService.UpdateUserSelf(c.Request.Context(), id, &req)
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

// AdminUpdateUser handles PATCH /users/:id/admin and accepts the privileged
// payload (role flags, activation status, …). Restricted to admins via the
// RequireAdmin middleware on the route group. Every successful call is
// audit-logged so privilege changes can be traced.
// @Summary Update a user (admin)
// @Description Admin-only update that may flip privileged fields
// @Tags users
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param id path string true "User ID"
// @Param input body api.UpdateUserRequest true "Privileged user update data"
// @Success 200 {object} interfaces.User
// @Router /users/{id}/admin [patch]
func (uh *UserHandler) AdminUpdateUser(c *gin.Context) {
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

	auditLog(c, "user.admin_update", map[string]any{
		"target_user_id":     id,
		"set_is_superuser":   req.IsSuperuser,
		"set_is_active":      req.IsActive,
		"set_email_verified": req.EmailVerified,
	})

	httpresp.NewResult(c, http.StatusOK, updatedUser)
}

// GetAdmins retrieves all users with an admin-level role.
// @Summary Get admin users
// @Description Get all users with admin, root or superadmin role
// @Tags users
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {array} interfaces.User "List of admin users"
// @Failure 500 {object} httpresp.HTTPError
// @Router /users/admins [get]
func (uh *UserHandler) GetAdmins(c *gin.Context) {
	admins := make([]*interfaces.User, 0)
	for _, role := range []string{"admin", "root", "superadmin"} {
		users, err := uh.userService.GetUsersByRole(c.Request.Context(), role)
		if err != nil {
			httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
			return
		}
		admins = append(admins, users...)
	}
	httpresp.NewResult(c, http.StatusOK, admins)
}

// GetCustomers retrieves all users with the "user" role (customers).
// @Summary Get customer users
// @Description Get all users with the user role (customers)
// @Tags users
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {array} interfaces.User "List of customers"
// @Failure 500 {object} httpresp.HTTPError
// @Router /users/customers [get]
func (uh *UserHandler) GetCustomers(c *gin.Context) {
	users, err := uh.userService.GetUsersByRole(c.Request.Context(), "user")
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpresp.NewResult(c, http.StatusOK, users)
}

// CreateAdmin creates a new user with the admin role.
// @Summary Create an admin user
// @Description Create a new user with admin role (admin-only)
// @Tags users
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param user body api.SignUpRequest true "Admin user information"
// @Success 201 {object} interfaces.User "Admin user created"
// @Failure 400 {object} gin.H "Validation error"
// @Failure 409 {object} gin.H "Email already in use"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /users/admins [post]
func (uh *UserHandler) CreateAdmin(c *gin.Context) {
	req := new(api.SignUpRequest)
	if err := c.ShouldBindBodyWith(req, binding.JSON); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := govalidator.ValidateStruct(req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	// Force the role to admin before creating the user
	req.Role = "admin"
	result, err := uh.authService.SignUpWithPassword(c.Request.Context(), req)
	if err != nil {
		httpresp.FromError(c, err)
		return
	}

	auditLog(c, "user.admin_create", map[string]any{
		"new_admin_email": req.Email,
		"new_admin_id":    result.User.ID.String(),
	})

	httpresp.NewResult(c, http.StatusCreated, result.User)
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

	auditLog(c, "user.admin_delete", map[string]any{"target_user_id": id})

	c.Status(http.StatusNoContent)
}
