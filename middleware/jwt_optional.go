package middleware

import (
	"encoding/json"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

// OptionalJWTMiddleware parses a Bearer token when present and attaches JWT
// claims to the context. Invalid or missing tokens do not block the request.
func OptionalJWTMiddleware(mw *jwt.GinJWTMiddleware) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := mw.GetClaimsFromJWT(c)
		if err != nil {
			c.Next()
			return
		}
		if !optionalClaimsNotExpired(mw, claims) {
			c.Next()
			return
		}

		c.Set("JWT_PAYLOAD", claims)
		identity := mw.IdentityHandler(c)
		if identity != nil {
			c.Set(mw.IdentityKey, identity)
		}
		c.Next()
	}
}

func optionalClaimsNotExpired(mw *jwt.GinJWTMiddleware, claims jwt.MapClaims) bool {
	switch v := claims[mw.ExpField].(type) {
	case nil:
		return false
	case float64:
		return int64(v) >= mw.TimeFunc().Unix()
	case json.Number:
		n, err := v.Int64()
		return err == nil && n >= mw.TimeFunc().Unix()
	default:
		return false
	}
}
