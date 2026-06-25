package middleware

import (
	"net/http"
	"strings"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"

	"github.com/malekradhouane/magic/utils/httpresp"
)

// adminRoles is the set of role names treated as administrators for
// ownership checks. Keep in sync with RequireAdmin().
var adminRoles = map[string]struct{}{
	"admin":      {},
	"root":       {},
	"superadmin": {},
}

// RequireSelfOrAdmin returns a middleware that lets the request through only
// when the JWT subject ("id" claim) matches the URL path parameter named
// paramName, or when the caller has an admin-level role. Must run AFTER the
// auth middleware so the JWT payload is populated.
func RequireSelfOrAdmin(paramName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := jwt.ExtractClaims(c)
		callerID, _ := claims["id"].(string)
		role, _ := claims["role"].(string)
		target := strings.TrimSpace(c.Param(paramName))

		if _, isAdmin := adminRoles[strings.ToLower(role)]; isAdmin {
			c.Next()
			return
		}
		if callerID == "" || target == "" || callerID != target {
			httpresp.NewErrorMessage(c, http.StatusForbidden, "forbidden")
			c.Abort()
			return
		}
		c.Next()
	}
}

// IsAdminRole returns true when the given role string is recognised as an
// administrator role by RequireAdmin / RequireSelfOrAdmin.
func IsAdminRole(role string) bool {
	_, ok := adminRoles[strings.ToLower(strings.TrimSpace(role))]
	return ok
}
