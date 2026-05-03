package middleware

import (
	"strings"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"

	"github.com/malekradhouane/magic/pkg/gatekeeper"
)

// DualTokenMiddleware validates Authorization bearer tokens issued either by gin-jwt or an OIDC provider.
// Order of checks:
// 1) Try to parse as a gin-jwt token without aborting the request. If valid, set gin-jwt style claims in context and continue.
// 2) If not valid, try to verify as an OIDC token using the provided verifier. If valid, map essential fields to gin-jwt style claims and continue.
// 3) Otherwise, fall back to the standard gin-jwt middleware (which may validate cookie-based sessions or return 401).
func DualTokenMiddleware(ginAuth *jwt.GinJWTMiddleware, verifier *gatekeeper.Verifier, sessionStore gatekeeper.IdentityStore, ginJWT *GinJWT) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cookie, err := c.Request.Cookie("magic_session"); err == nil {
			if sess, err := sessionStore.Get(c.Request.Context(), cookie.Value); err == nil {

				claims := jwt.MapClaims{
					ginJWT.IdentityKey: sess.UserInfo.Username,
					"role":             sess.UserInfo.Role,
				}

				c.Set("JWT_PAYLOAD", claims)
				c.Next()
				return
			}
		}
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			// 1) Try OIDC verification first to avoid HS256 signature errors on RS256 tokens
			if verifier != nil {
				if vt, err := verifier.VerifyToken(c.Request.Context(), authHeader); err == nil && vt != nil {
					claims := jwt.MapClaims{
						ginJWT.IdentityKey: vt.Claims.Username,
						"role":             vt.Claims.Role,
					}
					// Normalize into gin context as if it was a gin-jwt claim set
					c.Set("JWT_PAYLOAD", claims)
					c.Next()
					return
				}
			}

			// 2) Try gin-jwt token parsing (non-intrusive)
			if token, err := ginAuth.ParseToken(c); err == nil && token != nil && token.Valid {
				claims, err := ginAuth.GetClaimsFromJWT(c)
				if err == nil {
					// Normalize by setting the payload where gin-jwt expects it
					c.Set("JWT_PAYLOAD", claims)
					c.Next()
					return
				}
			}
		}

		// 3) Fallback to the standard gin-jwt middleware behavior
		ginAuth.MiddlewareFunc()(c)
	}
}
