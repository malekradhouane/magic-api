package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	jwttoken "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/markbates/goth/gothic"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/magic/configmanager"
	"github.com/malekradhouane/magic/middleware"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/service"
	"github.com/malekradhouane/magic/utils/httpresp"
)

type controller struct {
	cman             configmanager.ManagerContract
	authService      *service.AuthService
	auth             *jwt.GinJWTMiddleware
	ginJWT           *middleware.GinJWT
	loginRateLimit   gin.HandlerFunc
	genericRateLimit gin.HandlerFunc
}

// NewController wires the auth controller. loginRL is a combined IP + email
// rate limiter installed on /authenticate, /forgot-password and /reset-password
// to defeat brute-force / credential stuffing. genericRL falls back to an IP
// limiter when the body does not carry an email field.
func NewController(
	cman configmanager.ManagerContract,
	auth *jwt.GinJWTMiddleware,
	ginJWT *middleware.GinJWT,
	authService *service.AuthService,
	loginRL gin.HandlerFunc,
	genericRL gin.HandlerFunc,
) (*controller, error) {
	if cman == nil {
		return nil, fmt.Errorf("config manager is missing")
	}
	if loginRL == nil {
		return nil, fmt.Errorf("login rate limiter is required")
	}
	if genericRL == nil {
		genericRL = loginRL
	}

	return &controller{
		cman:             cman,
		auth:             auth,
		ginJWT:           ginJWT,
		authService:      authService,
		loginRateLimit:   loginRL,
		genericRateLimit: genericRL,
	}, nil
}

// SetupRoutes creates routes for the provided group
func (ctrl *controller) SetupRoutes(g *gin.RouterGroup) *gin.RouterGroup {
	// authenticate endpoint — strict IP + email rate limit.
	g.POST("/authenticate", ctrl.loginRateLimit, ctrl.auth.LoginHandler)

	// get identity of authenticated user
	g.GET("/identity", ctrl.auth.MiddlewareFunc(), ctrl.Identity)

	// Refresh time can be longer than token timeout
	g.POST("/refresh_token", ctrl.auth.MiddlewareFunc(), ctrl.auth.RefreshHandler)

	// Generate a permanent token for third-party app use — admin only to
	// prevent any authenticated user from minting long-lived tokens.
	g.GET("/generate-token", ctrl.auth.MiddlewareFunc(), middleware.RequireAdmin(), ctrl.GenerateToken)

	// logout endpoint
	g.POST("/logout", ctrl.auth.MiddlewareFunc(), ctrl.Logout)

	// activation endpoint — rate-limit by IP to slow token enumeration.
	g.GET("/activate/:token", ctrl.genericRateLimit, ctrl.ActivateAccount)

	// password reset endpoints — strict IP + email rate limit.
	g.POST("/forgot-password", ctrl.loginRateLimit, ctrl.ForgotPassword)
	g.POST("/reset-password", ctrl.genericRateLimit, ctrl.ResetPassword)

	g.GET("/auth/:provider", ctrl.StartAuth)
	g.GET("/auth/:provider/callback", ctrl.CompleteAuth)

	return g
}

// Authenticate ...
// @Summary Authenticate user
// @Description authenticate user using provided credentials. Returned token can be used as "Bearer [insert token here]" in the 'Authorize' form
// @Tags authenticate
// @ID authenticate-user-by-username-password
// @Accept  json
// @Produce  json
// @Param credential body interfaces.Credential true "Authentication credentials"
// @Success 200 {object} httpresp.JSONResult{success=bool,result=interfaces.Token}
// @Failure 401 {object} httpresp.HTTPError401
// @Failure 500 {object} httpresp.HTTPError
// // @Failure default {object} httpresp.HTTPError
// @Router /authenticate [post]
func Authenticate(c *gin.Context) {
	// here only because of swagger doc
}

func (ctrl *controller) Identity(c *gin.Context) {
	claims := jwt.ExtractClaims(c)
	if _, ok := claims[ctrl.ginJWT.IdentityKey]; !ok {
		httpresp.NewErrorMessage(c, http.StatusUnauthorized, "User is not authenticated!")
		return
	}

	getStr := func(key string) string {
		if v, ok := claims[key]; ok && v != nil {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}
	getBool := func(key string) bool {
		if v, ok := claims[key]; ok && v != nil {
			if b, ok := v.(bool); ok {
				return b
			}
		}
		return false
	}

	user := interfaces.Identity{
		ID:               getStr("id"),
		UserName:         getStr(ctrl.ginJWT.IdentityKey),
		FirstName:        getStr("firstName"),
		LastName:         getStr("lastName"),
		Email:            getStr("email"),
		Role:             getStr("role"),
		EmailVerified:    getBool("emailVerified"),
		ProfileCompleted: getBool("profileCompleted"),
		OrganizationID:   getStr("organizationID"),
	}

	httpresp.NewResult(c, http.StatusOK, user)
}

// Logout godoc
// @Summary Logout user
// @Description Logs out the authenticated user by invalidating the token
// @Tags authenticate
// @ID logout-user
// @Accept  json
// @Produce  json
// @Success 200 {object} httpresp.JSONResult "Successfully logged out"
// @Failure 401 {object} httpresp.HTTPError401 "Unauthorized"
// @Router /logout [post]
// @Security ApiKeyAuth
func (ctrl *controller) Logout(c *gin.Context) {
	// Extract claims to log the logout event
	claims := jwt.ExtractClaims(c)
	if username, ok := claims[ctrl.ginJWT.IdentityKey].(string); ok {
		ctrl.authService.LogLogout(c.Request.Context(), username)
	}

	// In a JWT stateless authentication system, the token cannot be directly invalidated
	// on the server side. The client should simply discard the token.
	// For a more secure implementation, you could:
	// 1. Maintain a blacklist of invalidated tokens
	// 2. Use short-lived tokens with refresh tokens
	// 3. Store token metadata in Redis with expiration

	httpresp.NewResult(c, http.StatusOK, gin.H{
		"message": "Successfully logged out",
		"note":    "Please discard the token on the client side",
	})
}

// RefreshToken refreshes the provided token
// @Summary RefreshToken refreshes authenticated user's token
// @Description refreshes the provided token
// @Tags authenticate
// @ID refresh-token
// @Accept  json
// @Produce  json
// @Param token query string true "Token"
// @Success 200 {object} httpresp.JSONResult
// @Failure 401 {object} httpresp.HTTPError401
// @Failure 500 {object} httpresp.HTTPError
// // @Failure default {object} httpresp.HTTPError
// @Router /refresh_token [post]
// @Security ApiKeyAuth
func (ctrl *controller) RefreshToken(c *gin.Context) {
	// here only because of swagger doc
}

// thirdPartyTokenTTL caps the lifetime of long-lived "permanent" tokens
// minted by GenerateToken. They are still much shorter than the previous
// 14 h value to limit blast radius if leaked. Override via env.
const thirdPartyTokenTTL = 1 * time.Hour

// claimString safely extracts a string claim. type-asserting without ok will
// panic when the claim is missing or null in the JWT — never let user input
// crash the handler.
func claimString(claims jwt.MapClaims, key string) string {
	if v, ok := claims[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (ctrl *controller) GenerateToken(c *gin.Context) {
	// extracts user claims
	claims := jwt.ExtractClaims(c)

	identity := &interfaces.Identity{
		ID:             claimString(claims, "id"),
		UserName:       claimString(claims, ctrl.ginJWT.IdentityKey),
		FirstName:      claimString(claims, "firstName"),
		LastName:       claimString(claims, "lastName"),
		Role:           claimString(claims, "role"),
		OrganizationID: claimString(claims, "organizationID"),
	}
	if identity.ID == "" {
		httpresp.NewErrorMessage(c, http.StatusUnauthorized, "user is not authenticated")
		return
	}

	token, err := ctrl.createToken(identity, time.Now().UTC().Add(thirdPartyTokenTTL))
	if err != nil {
		httpresp.NewError(c, http.StatusInternalServerError, err)
		return
	}

	httpresp.NewResult(c, http.StatusOK, token)
}

func (ctrl *controller) createToken(i *interfaces.Identity, expireAt time.Time) (string, error) {
	token := jwttoken.NewWithClaims(jwttoken.SigningMethodHS256, &jwttoken.MapClaims{
		"exp":                   expireAt.Unix(),
		"iat":                   time.Now().UTC().Unix(),
		"id":                    i.ID,
		"role":                  i.Role,
		"firstName":             i.FirstName,
		"lastName":              i.LastName,
		"organizationID":        i.OrganizationID,
		ctrl.ginJWT.IdentityKey: i.UserName,
	})

	// retrieves secret
	secret, _ := ctrl.cman.Magic().Customer["SECRET"].(string)
	if secret == "" {
		return "", fmt.Errorf("jwt secret not configured")
	}

	// Sign and get the complete encoded token as a string
	tokenString, err := token.SignedString([]byte(secret))

	return tokenString, err
}

func (ctrl *controller) StartAuth(c *gin.Context) {
	provider := c.Param("provider")
	// Set provider for gothic (so it knows which one to use). The gothic
	// library reads this value with a *string* key "provider"; do NOT change
	// the key type or gothic will silently fail to resolve the provider.
	//nolint:staticcheck // SA1029: gothic API requires a plain-string context key.
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "provider", provider))
	gothic.BeginAuthHandler(c.Writer, c.Request)
}

func (ctrl *controller) CompleteAuth(c *gin.Context) {
	provider := c.Param("provider")
	//nolint:staticcheck // SA1029: gothic API requires a plain-string context key.
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "provider", provider))

	// Resolve the frontend URL for redirect (after OAuth completion)
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	callbackPath := frontendURL + "/auth/callback"

	gothUser, err := gothic.CompleteUserAuth(c.Writer, c.Request)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, callbackPath+"?error="+url.QueryEscape(err.Error()))
		return
	}

	signUpReq := &api.SignUpRequest{
		Email:      gothUser.Email,
		Username:   gothUser.NickName,
		FirstName:  gothUser.FirstName,
		LastName:   gothUser.LastName,
		AvatarURL:  gothUser.AvatarURL,
		Provider:   provider,
		ProviderID: gothUser.UserID,
		Role:       "user",
	}
	user, err := ctrl.authService.SignUpWithOAuth(c.Request.Context(), signUpReq)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, callbackPath+"?error="+url.QueryEscape(err.Error()))
		return
	}

	expireAt := time.Now().UTC().Add(thirdPartyTokenTTL)
	token, err := ctrl.createToken(&interfaces.Identity{
		UserName:      user.User.Username,
		FirstName:     user.User.FirstName,
		LastName:      user.User.LastName,
		ID:            user.User.ID.String(),
		Email:         user.User.Email,
		EmailVerified: user.User.EmailVerified,
		Role:          user.User.Role,
	}, expireAt)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, callbackPath+"?error="+url.QueryEscape(err.Error()))
		return
	}

	// Redirect to the frontend callback page with the tokens in query params
	redirectURL := fmt.Sprintf(
		"%s?token=%s&refresh_token=%s&expire=%s",
		callbackPath,
		url.QueryEscape(token),
		url.QueryEscape(token),
		url.QueryEscape(expireAt.Format(time.RFC3339)),
	)
	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// ActivateAccount godoc
// @Summary Activate user account
// @Description Activates a user account using the validation token from email
// @Tags authenticate
// @ID activate-account
// @Accept  json
// @Produce  json
// @Param token path string true "Activation token"
// @Success 200 {object} httpresp.JSONResult "Account activated successfully"
// @Failure 400 {object} httpresp.HTTPError "Invalid or expired token"
// @Router /activate/{token} [get]
func (ctrl *controller) ActivateAccount(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, "Activation token is required")
		return
	}

	err := ctrl.authService.ActivateAccount(c.Request.Context(), token)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	httpresp.NewResult(c, http.StatusOK, gin.H{
		"message": "Account activated successfully",
	})
}

// ForgotPassword godoc
// @Summary Request password reset
// @Description Sends a password reset email to the user
// @Tags authenticate
// @ID forgot-password
// @Accept  json
// @Produce  json
// @Param email body map[string]string true "Email address"
// @Success 200 {object} httpresp.JSONResult "Password reset email sent"
// @Failure 400 {object} httpresp.HTTPError "Invalid request"
// @Router /forgot-password [post]
func (ctrl *controller) ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, "Valid email is required")
		return
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	err := ctrl.authService.RequestPasswordReset(c.Request.Context(), req.Email, frontendURL)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Always return success to prevent email enumeration
	httpresp.NewResult(c, http.StatusOK, gin.H{
		"message": "If an account with that email exists, a password reset link has been sent",
	})
}

// ResetPassword godoc
// @Summary Reset password
// @Description Resets the user's password using the token
// @Tags authenticate
// @ID reset-password
// @Accept  json
// @Produce  json
// @Param request body map[string]string true "Token and new password"
// @Success 200 {object} httpresp.JSONResult "Password reset successfully"
// @Failure 400 {object} httpresp.HTTPError "Invalid or expired token"
// @Router /reset-password [post]
func (ctrl *controller) ResetPassword(c *gin.Context) {
	var req struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	err := ctrl.authService.ResetPassword(c.Request.Context(), req.Token, req.NewPassword)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	httpresp.NewResult(c, http.StatusOK, gin.H{
		"message": "Password reset successfully",
	})
}
