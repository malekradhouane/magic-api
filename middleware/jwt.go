package middleware

import (
	"errors"
	"os"
	"strconv"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/magic/configmanager"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/store/types"
	"github.com/malekradhouane/magic/utils/httpresp"
)

// ErrJWTSecretMissing is returned when no JWT secret is configured.
var ErrJWTSecretMissing = errors.New("JWT secret is not configured: set MAGIC_CUSTOMER_SECRET or customer.SECRET in config")

// ErrJWTSecretTooShort is returned when the configured JWT secret is shorter
// than the NIST SP 800-131A recommendation of 32 bytes for HS256.
var ErrJWTSecretTooShort = errors.New("JWT secret is too short (< 32 bytes): generate one with `openssl rand -base64 48`")

type GinJWT struct {
	cman        configmanager.ManagerContract
	userStore   types.UserStore
	jwtSecret   string
	IdentityKey string
}

func NewGinJwt(cman configmanager.ManagerContract, userStore types.UserStore) (*GinJWT, error) {
	return &GinJWT{
		cman:        cman,
		userStore:   userStore,
		jwtSecret:   "",
		IdentityKey: "id",
	}, nil
}

// Token TTL defaults. 15 minutes for access follows OWASP guidance for stateless
// bearer tokens; the refresh window is longer so users do not need to re-enter
// credentials between sessions.
const (
	defaultAccessTokenTTL = 15 * time.Minute
	defaultRefreshWindow  = 7 * 24 * time.Hour
)

// envDuration parses an integer-of-seconds env var and returns the duration
// or the fallback when the value is missing / invalid / non-positive.
func envDuration(name string, fallback time.Duration) time.Duration {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	secs, err := strconv.Atoi(v)
	if err != nil || secs <= 0 {
		return fallback
	}
	return time.Duration(secs) * time.Second
}

// ResolveSecret extracts the JWT signing secret from configuration and
// validates it. Returns a typed error rather than terminating the process so
// the caller in main() can decide whether to fail-fast.
func (x *GinJWT) ResolveSecret() (string, error) {
	customerConfig := x.cman.Magic().Customer

	// koanf lowercases env-var keys, so check both casings.
	secret, _ := customerConfig["SECRET"].(string)
	if secret == "" {
		secret, _ = customerConfig["secret"].(string)
	}
	if secret == "" || secret == "change-me-in-production" {
		return "", ErrJWTSecretMissing
	}
	if len(secret) < 32 {
		return "", ErrJWTSecretTooShort
	}
	return secret, nil
}

// MiddlewareHandler builds the configured gin-jwt middleware. It returns an
// error when the JWT secret cannot be resolved instead of terminating the
// process; the caller in main() is expected to abort startup on error.
func (x *GinJWT) MiddlewareHandler() (*jwt.GinJWTMiddleware, error) {
	accessTTL := envDuration("MAGIC_JWT_ACCESS_TTL_SECONDS", defaultAccessTokenTTL)
	refreshWindow := envDuration("MAGIC_JWT_REFRESH_TTL_SECONDS", defaultRefreshWindow)

	secret, err := x.ResolveSecret()
	if err != nil {
		return nil, err
	}

	return &jwt.GinJWTMiddleware{
		Realm:       "magic",
		Key:         []byte(secret),
		Timeout:     accessTTL,
		MaxRefresh:  refreshWindow,
		IdentityKey: x.IdentityKey,
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if v, ok := data.(*interfaces.Identity); ok {
				return jwt.MapClaims{
					x.IdentityKey:      v.ID,
					"userName":         v.UserName,
					"firstName":        v.FirstName,
					"lastName":         v.LastName,
					"id":               v.ID,
					"role":             v.Role,
					"email":            v.Email,
					"emailVerified":    v.EmailVerified,
					"profileCompleted": v.ProfileCompleted,
				}
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			identity := &interfaces.Identity{}
			if id, ok := claims["id"].(string); ok {
				identity.ID = id
			}
			if userName, ok := claims["userName"].(string); ok {
				identity.UserName = userName
			}
			if firstName, ok := claims["firstName"].(string); ok {
				identity.FirstName = firstName
			}
			if lastName, ok := claims["lastName"].(string); ok {
				identity.LastName = lastName
			}
			if role, ok := claims["role"].(string); ok {
				identity.Role = role
			}
			if email, ok := claims["email"].(string); ok {
				identity.Email = email
			}
			if emailVerified, ok := claims["emailVerified"].(bool); ok {
				identity.EmailVerified = emailVerified
			}
			if profileCompleted, ok := claims["profileCompleted"].(bool); ok {
				identity.ProfileCompleted = profileCompleted
			}
			return identity
		},
		Authenticator: func(c *gin.Context) (interface{}, error) {
			var creds interfaces.Credential
			if err := c.ShouldBind(&creds); err != nil {
				return "", jwt.ErrMissingLoginValues
			}
			return x.login(c, creds)
		},
		Authorizator: func(data interface{}, c *gin.Context) bool {
			_, ok := data.(*interfaces.Identity)
			return ok
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			httpresp.NewErrorMessage(c, code, message)
		},
		LoginResponse: func(c *gin.Context, code int, token string, exp time.Time) {
			t := interfaces.Token{AccessToken: token, RefreshToken: token, Expiration: exp}
			httpresp.NewResult(c, code, t)
		},
		RefreshResponse: func(c *gin.Context, code int, token string, exp time.Time) {
			t := interfaces.Token{AccessToken: token, RefreshToken: token, Expiration: exp}
			httpresp.NewResult(c, code, t)
		},
		LogoutResponse: func(c *gin.Context, code int) {
			// handled by function Logout
		},
		TokenLookup:   "header:Authorization",
		TokenHeadName: "Bearer",
		TimeFunc:      time.Now,
	}, nil
}

func (x *GinJWT) login(c *gin.Context, creds interfaces.Credential) (*interfaces.Identity, error) {
	user, err := x.userStore.Authenticate(c.Request.Context(), &creds)
	if err != nil {
		return nil, err
	}

	identity := &interfaces.Identity{
		ID:            user.ID.String(),
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		UserName:      user.Username,
		Role:          user.Role,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
	}
	return identity, nil
}

// LoggerWithUsername logs every HTTP request with the authenticated username
// (or "unlogged request" when no JWT claims are attached).
//
// TODO: switch to slog (stdlib) once the codebase migration is complete; also
// accept the logger as a constructor dependency instead of allocating one
// per call.
func (x *GinJWT) LoggerWithUsername() gin.HandlerFunc {
	log := logrus.New()
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		loggedUser := "unlogged request"
		claims := jwt.ExtractClaims(c)
		if username, ok := claims[x.IdentityKey].(string); ok {
			loggedUser = username
		}

		end := time.Now()
		latency := end.Sub(start)

		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		if raw != "" {
			path = path + "?" + raw
		}

		log.Infof(" %v | %3d | %13v | %15s | %-7s %s | %s\n",
			end.Format("2006/01/02 - 15:04:05"),
			statusCode,
			latency,
			clientIP,
			method,
			path,
			loggedUser,
		)
	}
}
