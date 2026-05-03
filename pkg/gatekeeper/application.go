package gatekeeper

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"github.com/malekradhouane/magic/internal/core"
	"github.com/malekradhouane/magic/pkg/gatekeeper/casbin"
	"github.com/malekradhouane/magic/pkg/gatekeeper/oidc"
	"github.com/malekradhouane/magic/utils/httpresp"
)

// Constants for error codes and metrics
const (
	ErrPKCEGeneration = "failed_to_generate_pkce"
	ErrStateSave      = "failed_to_save_state"
	ErrAuthBegin      = "failed_to_begin_auth"
	ErrInvalidState   = "invalid_state_parameter"
	ErrInvalidURL     = "invalid_url"
)

// NewApplicationParams contains the dependencies required to create a new gatekeeper application
type NewApplicationParams struct {
	Logger        core.LoggerContract
	Authorization *casbin.CasbinAuth
	OIDCProvider  oidc.OIDCProvider
	StateStore    StateStore
	SessionStore  IdentityStore
	Config        Config
	CookieConfig  CookieConfig
	BaseURL       string
}

// application is the main gatekeeper application struct
type application struct {
	logger        core.LoggerContract
	authorization *casbin.CasbinAuth
	oidcProvider  oidc.OIDCProvider
	stateStore    StateStore
	sessionStore  IdentityStore
	config        Config
	cookieConfig  CookieConfig
	baseURL       string
}

// NewApplication creates a new instance of the gatekeeper application
func NewApplication(params NewApplicationParams) (*application, error) {
	if params.Config.AuthExtra == nil {
		params.Config.AuthExtra = make(map[string]string)
	}

	if params.Config.StateTTL <= 0 {
		params.Config.StateTTL = 10 * time.Minute
	} else if params.Config.StateTTL > 24*time.Hour {
		return nil, errors.New("state TTL cannot exceed 24 hours")
	}

	if params.Config.SessionTTL == 0 {
		params.Config.SessionTTL = 60 * time.Minute
	}

	if params.CookieConfig.Domain == "" {
		params.CookieConfig.Domain = "/"
	}
	if !params.CookieConfig.Secure {
		params.CookieConfig.Secure = true
	}
	if !params.CookieConfig.HTTPOnly {
		params.CookieConfig.HTTPOnly = true
	}
	if params.CookieConfig.SameSite == "" {
		params.CookieConfig.SameSite = "Lax"
	}
	if params.CookieConfig.SessionTTL == 0 {
		params.CookieConfig.SessionTTL = 60 * time.Minute
	}

	app := &application{
		logger:        params.Logger,
		authorization: params.Authorization,
		oidcProvider:  params.OIDCProvider,
		stateStore:    params.StateStore,
		sessionStore:  params.SessionStore,
		config:        params.Config,
		cookieConfig:  params.CookieConfig,
		baseURL:       params.BaseURL,
	}

	var errs []string
	if app.logger == nil {
		errs = append(errs, "logger is missing")
	}
	if app.oidcProvider == nil {
		errs = append(errs, "OIDC provider is missing")
	}
	if app.stateStore == nil {
		errs = append(errs, "state store is missing")
	}
	if app.config.DefaultRedirectURL == "" {
		errs = append(errs, "config.DefaultRedirectURL is missing")
	}

	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "; "))
	}

	return app, nil
}

// Authorize middleware checks if the subject is authorized for the given action on the object
func (app *application) Authorize(obj string, act string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var roleName string
		var ok bool
		// try to get user from session
		if cookie, err := c.Request.Cookie("magic_session"); err == nil && cookie.Value != "" {
			sess, err := app.sessionStore.Get(c.Request.Context(), cookie.Value)
			if err != nil {
				httpresp.NewErrorMessage(c, http.StatusUnauthorized, "Role not found in claims")
				return
			}
			roleName = sess.UserInfo.Role
			ok = true
		}
		if !ok {
			claims := jwt.ExtractClaims(c)
			if roleName, ok = claims["role"].(string); !ok {
				httpresp.NewErrorMessage(c, http.StatusUnauthorized, "Role not found in claims")
				return
			}
		}

		authorized, err := app.authorization.Authorization().Enforce(strings.ToLower(roleName), obj, act)
		if !authorized {
			c.AbortWithStatusJSON(
				http.StatusForbidden,
				gin.H{
					"success": false,
					"error":   "user not authorized",
				},
			)
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				gin.H{
					"success": false,
					"error":   err.Error(),
				},
			)
			return
		}
		c.Next()
	}
}

// isValidURL validates a URL string
func (app *application) isValidURL(urlStr string) bool {
	if urlStr == "" {
		return false
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	return u.Scheme != "" && u.Host != ""
}

// LoginHandler initiates the OIDC login flow
func (app *application) LoginHandler(c *gin.Context) {
	// Validate state parameter
	var err error
	state := c.Query("state")
	if state == "" {
		if state, err = app.stateStore.GenerateState(c.Request.Context()); err != nil {
			app.logger.Error("Failed to generate state", "error", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Authentication failed",
			})
			return
		}
	}

	if len(state) < 16 || len(state) > 128 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid state parameter length (16-128 characters required)",
			"code":    ErrInvalidState,
		})
		return
	}

	// Set up context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Determine redirect URLs
	redirectURL := c.Query("redirect_url")
	if redirectURL == "" {
		redirectURL = app.config.DefaultRedirectURL
	} else if !app.isValidURL(redirectURL) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid redirect URL",
			"code":    ErrInvalidURL,
		})
		return
	}

	continueURL := c.Query("continue_url")
	if continueURL == "" {
		continueURL = "/"
	}

	// Generate PKCE verifier and challenge
	verifier, challenge := oidc.GeneratePKCE()

	// Generate OIDC nonce
	nonce := oidc.GenerateNonce()

	// Prepare and save state
	stateData := StateData{
		CodeVerifier: verifier,
		RedirectURL:  redirectURL,
		ContinueURL:  continueURL,
		CreatedAt:    time.Now(),
		Provider:     "default",
		Nonce:        nonce,
	}

	if err := app.stateStore.Save(ctx, state, stateData, app.config.StateTTL); err != nil {
		app.logger.Error("error", err, "path", c.Request.URL.Path)

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Authentication failed",
		})
		return
	}
	extra := make(map[string]string)
	for k, v := range app.config.AuthExtra {
		extra[k] = v
	}
	extra["nonce"] = nonce
	extra["scope"] = strings.Join(app.config.Scopes, " ")
	// Begin authentication with OIDC provider
	authURL, err := app.oidcProvider.BeginAuth(
		ctx,
		state,
		redirectURL,
		challenge,
		"S256",
		extra,
	)
	if err != nil {
		app.logger.Error("error", err, "path", c.Request.URL.Path)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Authentication failed",
		})
		return
	}

	// Redirect to OIDC provider
	c.Redirect(http.StatusFound, authURL)
}

// CallbackHandler handles the OIDC callback
func (app *application) CallbackHandler(c *gin.Context) {
	state := c.Query("state")
	code := c.Query("code")
	if state == "" || code == "" {
		app.logger.Error("Missing required parameters in callback",
			"state", state,
			"code", code != "")
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing required parameters",
		})
		return
	}

	stateData, err := app.stateStore.VerifyAndLoad(c.Request.Context(), state)
	if err != nil {
		app.logger.Error("State verification failed",
			"error", err.Error(),
			"state", state)

		var status int
		var message string

		switch {
		case errors.Is(err, ErrStateNotFound):
			status = http.StatusBadRequest
			message = "Invalid state parameter"
		case errors.Is(err, ErrStateExpired):
			status = http.StatusBadRequest
			message = "Authentication session expired"
		default:
			status = http.StatusInternalServerError
			message = "Authentication failed"
		}

		c.AbortWithStatusJSON(status, gin.H{
			"success": false,
			"error":   message,
		})
		return
	}

	token, err := app.oidcProvider.ExchangeToken(
		c.Request.Context(),
		code,
		stateData.CodeVerifier,
		stateData.RedirectURL,
	)
	if err != nil {
		app.logger.Error("Token exchange failed",
			"error", err.Error(),
			"code", code,
			"redirect_uri", stateData.RedirectURL)

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Token exchange failed",
		})
		return
	}

	app.logger.Debug("Token exchange successful",
		"access_token_expires", token.Expiry,
		"token_type", token.TokenType)

	fmt.Println("ID-token", token.IDToken)
	fmt.Println("token", token.AccessToken)

	if _, err := app.oidcProvider.VerifyIDToken(c.Request.Context(), token.IDToken); err != nil {
		app.logger.Error("ID token verification failed",
			"error", err.Error(),
			"id_token", token.IDToken)

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "ID token verification failed",
		})
		return
	}

	// Verify OIDC nonce matches the stored one
	if stateData.Nonce != "" {
		claims, err := app.oidcProvider.GetTokenClaims(c.Request.Context(), token.IDToken)
		if err != nil {
			app.logger.Error("Failed to extract ID token claims", "error", err.Error())
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to extract ID token claims",
			})
			return
		}
		if claims.Nonce != "" && claims.Nonce != stateData.Nonce {
			app.logger.Error("Nonce verification failed", "expected", stateData.Nonce, "actual", claims.Nonce)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Invalid authentication response",
			})
			return
		}
	}

	userInfo, err := app.oidcProvider.GetUserInfo(c.Request.Context(), token.AccessToken, token.IDToken)
	if err != nil {
		app.logger.Error("Failed to get user info",
			"error", err.Error(),
			"access_token", token.AccessToken)

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get user information",
		})
		return
	}

	app.logger.Info("User authenticated",
		"user_id", userInfo.Subject,
		"email", userInfo.Email,
		"issuer", userInfo.Issuer)

	oauthToken := &oauth2.Token{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry,
	}

	if err := app.createSession(c, userInfo, oauthToken); err != nil {
		app.logger.Error("Session creation failed",
			"error", err.Error(),
			"user_id", userInfo.Subject)

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Session creation failed",
		})
		return
	}

	app.logger.Info("Authentication successful",
		"user", userInfo.Subject,
		"provider", userInfo.Issuer)

	c.Redirect(http.StatusFound, stateData.ContinueURL)
}

func (app *application) createSession(
	c *gin.Context,
	userInfo oidc.UserInfo,
	token *oauth2.Token,
) error {
	sessionToken, err := app.generateSessionToken(userInfo.Subject)
	if err != nil {
		app.logger.Error("failed to generate session token", "error", err)
		return fmt.Errorf("failed to create session: %w", err)
	}

	expiry := token.Expiry
	if expiry.IsZero() || expiry.Before(time.Now()) {
		expiry = time.Now().Add(app.config.SessionTTL)
	}
	sessionData := SessionData{
		UserInfo: oidc.UserInfo{
			Subject:  userInfo.Subject,
			Email:    userInfo.Email,
			Username: userInfo.Username,
			Role:     userInfo.Role,
		},
		TokenExpiry: expiry,
		CreatedAt:   time.Now(),
		LastUsedAt:  time.Now(),
		UserAgent:   c.Request.UserAgent(),
		ClientIP:    c.ClientIP(),
	}

	ttl := time.Until(expiry)
	if ttl > app.config.SessionTTL {
		ttl = app.config.SessionTTL
	}

	err = app.sessionStore.Set(c.Request.Context(), sessionToken, sessionData, ttl)
	if err != nil {
		app.logger.Error("failed to store session",
			"error", err,
			"session_token", sessionToken,
			"ttl_seconds", int(ttl.Seconds()))
		return fmt.Errorf("failed to store session: %w", err)
	}

	cookie := &http.Cookie{
		Name:     "magic_session",
		Value:    sessionToken,
		Path:     "/",
		Domain:   app.cookieConfig.Domain,
		Expires:  expiry,
		MaxAge:   int(ttl.Seconds()),
		Secure:   app.cookieConfig.Secure,
		HttpOnly: app.cookieConfig.HTTPOnly,
		SameSite: http.SameSiteLaxMode,
	}

	http.SetCookie(c.Writer, cookie)

	c.Set("user", sessionData.UserInfo)

	app.logger.Info("session created successfully",
		"user_id", userInfo.Subject,
		"session_token", sessionToken,
		"expires_at", expiry)

	return nil
}

// generateSessionToken creates a cryptographically secure session token
func (app *application) generateSessionToken(userID string) (string, error) {
	// Build payload in the same format as identity_store.generateToken: "<subject>:<unix>"
	timestamp := time.Now().Unix()
	payload := fmt.Sprintf("%s:%d", userID, timestamp)

	// Create HMAC signature using the configured session secret
	mac := hmac.New(sha256.New, []byte(app.cookieConfig.SessionSecret))
	if _, err := mac.Write([]byte(payload)); err != nil {
		return "", fmt.Errorf("failed to create HMAC: %w", err)
	}
	signature := mac.Sum(nil)

	// Return base64 encoded token: payload||signature
	return base64.URLEncoding.EncodeToString(append([]byte(payload), signature...)), nil
}

// verifySessionToken verifies the integrity of a session token
func (app *application) verifySessionToken(token string, sessionData *SessionData) error {
	// Decode the token
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return fmt.Errorf("failed to decode token: %w", err)
	}

	// Split payload and signature: last 32 bytes are HMAC-SHA256 signature.
	// Require at least 1 byte payload + 32 byte signature (33 total).
	if len(decoded) < 33 {
		return errors.New("invalid token format")
	}

	payload := decoded[:len(decoded)-32]
	expectedSignature := decoded[len(decoded)-32:]

	mac := hmac.New(sha256.New, []byte(app.cookieConfig.SessionSecret))
	_, err = mac.Write(payload)
	if err != nil {
		return fmt.Errorf("failed to verify HMAC: %w", err)
	}
	actualSignature := mac.Sum(nil)

	if !hmac.Equal(expectedSignature, actualSignature) {
		return errors.New("invalid token signature")
	}

	// Verify user ID matches
	parts := strings.Split(string(payload), ":")
	if len(parts) < 2 {
		return errors.New("invalid token format")
	}

	if parts[0] != sessionData.UserInfo.Subject {
		return errors.New("token user mismatch")
	}

	return nil
}
