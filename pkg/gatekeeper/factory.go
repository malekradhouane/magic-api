package gatekeeper

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/malekradhouane/magic/internal/core"
	"github.com/malekradhouane/magic/pkg/assertor"
	"github.com/malekradhouane/magic/pkg/gatekeeper/casbin"
	"github.com/malekradhouane/magic/pkg/gatekeeper/oidc"
)

// NewGateKeeperParams contains all dependencies required to create a new Gatekeeper instance
type NewGateKeeperParams struct {
	Logger                core.LoggerContract // Required
	GinRouter             *gin.Engine         // Required for default controller
	GateKeeperApplication ApplicationContract // Required
	Enforcer              EnforcerDependency  // Required
	OIDCProvider          oidc.OIDCProvider   // Optional for OIDC flow
	StateStore            StateStore          // Optional, will use default if nil
	Config                Config              // Optional, will use defaults if nil
	IdentityStore         IdentityStore       // Optional for session management
	OIDCVerifier          *Verifier           // Optional for token verification
	CookieConfig          CookieConfig        // Optional cookie configuration
	BaseURL               string              // Optional base URL for redirects
}

// Config holds gatekeeper configuration
type Config struct {
	StateTTL           time.Duration
	DefaultRedirectURL string
	AuthExtra          map[string]string
	Scopes             []string
	SessionTTL         time.Duration
}

// CookieConfig holds cookie configuration
type CookieConfig struct {
	Domain        string
	Secure        bool
	HTTPOnly      bool
	SameSite      string
	SessionTTL    time.Duration
	SessionSecret string
}

type ClaimsConfig struct {
	UsernameClaim string
	RoleClaim     string
}

// feature implements the GatekeeperContract
type feature struct {
	logger        core.LoggerContract
	application   ApplicationContract
	closers       []core.Closer
	oidcVerifier  *Verifier
	identityStore IdentityStore
	enforcer      EnforcerDependency
	config        Config
	cookieConfig  CookieConfig
}

// NewGateKeeper creates a new Gatekeeper instance with all dependencies
func NewGateKeeper(params NewGateKeeperParams) (*feature, error) {
	feat := &feature{
		logger:        params.Logger,
		application:   params.GateKeeperApplication,
		oidcVerifier:  params.OIDCVerifier,
		identityStore: params.IdentityStore,
		enforcer:      params.Enforcer,
		closers:       nil,
		config:        params.Config,
		cookieConfig:  params.CookieConfig,
	}

	// Set default values if not provided
	if feat.config.StateTTL == 0 {
		feat.config.StateTTL = 10 * time.Minute
	}
	if feat.config.DefaultRedirectURL == "" {
		feat.config.DefaultRedirectURL = "/api/callback"
	}
	if feat.config.AuthExtra == nil {
		feat.config.AuthExtra = make(map[string]string)
	}
	if feat.config.SessionTTL == 0 {
		feat.config.SessionTTL = 60 * time.Minute
	}

	// Set default cookie config if not provided
	if feat.cookieConfig.Domain == "" {
		feat.cookieConfig.Domain = "app.example.com"
	}
	if feat.cookieConfig.Secure == false {
		feat.cookieConfig.Secure = true
	}
	if feat.cookieConfig.HTTPOnly == false {
		feat.cookieConfig.HTTPOnly = true
	}
	if feat.cookieConfig.SameSite == "" {
		feat.cookieConfig.SameSite = "Lax"
	}
	if feat.cookieConfig.SessionTTL == 0 {
		feat.cookieConfig.SessionTTL = 60 * time.Minute
	}

	// Validate required parameters
	v := assertor.New()
	v.Assert(feat.logger != nil, "logger is missing")
	v.Assert(params.Enforcer != nil, "enforcer is missing")

	if err := v.Validate(); err != nil {
		return nil, err
	}

	// Create Casbin authorization
	auth, err := casbin.NewCasbinAuthorization(casbin.NewCasbinParams{
		Logger:   feat.logger.With("component", "casbin"),
		Enforcer: params.Enforcer,
	})
	if err != nil {
		feat.logger.Error("failed to create casbin authorization", "error", err)
		return nil, err
	}

	// Create state store if not provided
	stateStore := params.StateStore
	if stateStore == nil {
		stateStore = NewMemoryStateStore()
	}

	// Create identity store if not provided
	identityStore := params.IdentityStore
	if identityStore == nil {
		identityStore = NewIdentityStore(time.Hour, "", "", ClaimsConfig{}, CookieConfig{})
	}

	// Create OIDC verifier if not provided
	oidcVerifier := params.OIDCVerifier
	if oidcVerifier == nil && params.OIDCProvider != nil {
		oidcVerifier = NewVerifier(params.OIDCProvider)
	}

	// Create application
	appParams := NewApplicationParams{
		Logger:        feat.logger.With("component", "application"),
		Authorization: auth,
		OIDCProvider:  params.OIDCProvider,
		StateStore:    stateStore,
		SessionStore:  identityStore,
		Config:        feat.config,
		CookieConfig:  feat.cookieConfig,
		BaseURL:       params.BaseURL,
	}

	app, err := NewApplication(appParams)
	if err != nil {
		feat.logger.Error("failed to create application", "error", err)
		return nil, err
	}

	feat.application = app
	feat.closers = append(feat.closers, auth)
	feat.identityStore = identityStore
	feat.oidcVerifier = oidcVerifier

	controllerParams := NewControllerHttpGinParams{
		GinRouter:    params.GinRouter,
		App:          app,
		SessionStore: identityStore,
		Verifier:     oidcVerifier,
	}

	ctrl, err := NewControllerHttpGin(controllerParams)
	if err != nil {
		feat.logger.Error(err.Error())
		err = errors.Join(err, feat.Close(context.Background()))
		return nil, err
	}

	feat.closers = append(feat.closers, ctrl)

	return feat, nil
}

// Application returns the application instance
func (f *feature) Application() ApplicationContract {
	return f.application
}

// GinAuthMiddleware returns a Gin middleware for OIDC authentication
func (f *feature) GinAuthMiddleware() (gin.HandlerFunc, error) {
	if f.oidcVerifier == nil {
		return nil, errors.New("OIDC verifier is required for auth middleware")
	}

	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			f.logger.Warn("missing authorization header")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			return
		}

		// Verify the token
		token, err := f.oidcVerifier.VerifyToken(c.Request.Context(), authHeader)
		if err != nil {
			f.logger.Error("token verification failed", "error", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
			})
			return
		}

		// Set user in context
		c.Set("user", token.Claims)
		c.Set("token", token.AccessToken)
		c.Next()
	}, nil
}

func (f *feature) Authorize(obj string, act string) gin.HandlerFunc {
	return f.application.Authorize(obj, act)
}

// Close implements core.Closer
func (f *feature) Close(ctx context.Context) error {
	var err error
	for _, closer := range slices.Backward(f.closers) {
		err = errors.Join(err, closer.Close(ctx))
	}
	f.closers = nil
	return err
}

// Verifier handles OIDC token verification
type Verifier struct {
	provider oidc.OIDCProvider
}

// NewVerifier creates a new token verifier
func NewVerifier(provider oidc.OIDCProvider) *Verifier {
	return &Verifier{
		provider: provider,
	}
}

// VerifyToken verifies an OIDC token
func (v *Verifier) VerifyToken(ctx context.Context, header string) (*oidc.VerifiedToken, error) {
	if !strings.HasPrefix(header, "Bearer ") {
		return nil, fmt.Errorf("invalid authorization scheme: %s", header)
	}

	raw := strings.TrimPrefix(header, "Bearer ")
	// Verify the token using the OIDC provider
	token, err := v.provider.VerifyIDToken(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	// Extract claims
	claims, err := v.provider.GetTokenClaims(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to extract claims: %w", err)
	}

	return &oidc.VerifiedToken{
		AccessToken: raw,
		Claims:      claims,
	}, nil
}
