package middleware

import (
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/magic/configmanager"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/store/types"
	"github.com/malekradhouane/magic/utils/httpresp"
)

type GinJWT struct {
	cman             configmanager.ManagerContract
	userStore        types.UserStore
	jwtSecret        string
	defaultJWTSecret string
	IdentityKey      string
}

func NewGinJwt(cman configmanager.ManagerContract, userStore types.UserStore) (*GinJWT, error) {
	return &GinJWT{
		cman:             cman,
		userStore:        userStore,
		jwtSecret:        "",
		defaultJWTSecret: "change-me-in-production",
		IdentityKey:      "id",
	}, nil
}

// GinJwtMiddlewareHandler handles authentication
func (x *GinJWT) MiddlewareHandler() *jwt.GinJWTMiddleware {
	const defaultTimeout = 15 * time.Hour

	customerConfig := x.cman.Magic().Customer

	// retrieves secret
	secret, _ := customerConfig["SECRET"].(string)
	if secret == "" {
		secret = x.defaultJWTSecret
	}

	return &jwt.GinJWTMiddleware{
		Realm:       "test zone",
		Key:         []byte(secret),
		Timeout:     defaultTimeout,
		MaxRefresh:  defaultTimeout * 2,
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
			if _, ok := data.(*interfaces.Identity); ok {
				return true
			}

			return false
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
		// TokenLookup is a string in the form of "<source>:<name>" that is used
		// to extract token from the request.
		// Optional. Default value "header:Authorization".
		// Possible values:
		// - "header:<name>"
		// - "query:<name>"
		// - "cookie:<name>"
		// - "param:<name>"
		TokenLookup: "header: Authorization",
		// TokenLookup: "query:token",
		// TokenLookup: "cookie:token",

		// TokenHeadName is a string in the header. Default value is "Bearer"
		TokenHeadName: "Bearer",

		// TimeFunc provides the current time. You can override it to use another time value. This is useful for testing or if your server uses a different time zone than your tokens.
		TimeFunc: time.Now,
	}
}

func (x *GinJWT) login(c *gin.Context, creds interfaces.Credential) (*interfaces.Identity, error) {
	user, err := x.userStore.Authenticate(c, &creds)
	if err != nil {
		return nil, err
	}

	identity := &interfaces.Identity{
		ID:        user.ID.String(),
		UserName:  user.Username,
		Role:      user.Role,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}
	return identity, nil
}

// LoggerWithUsername logs the request with the username
func (x *GinJWT) LoggerWithUsername() gin.HandlerFunc {
	return func(c *gin.Context) {
		log := logrus.New()
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		var loggedUser string
		claims := jwt.ExtractClaims(c)

		if username, isUserNameOK := claims[x.IdentityKey].(string); isUserNameOK {
			loggedUser = username
		} else {
			loggedUser = "unlogged request"
		}

		// Stop timer
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
