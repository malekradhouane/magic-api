package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders sets a baseline of recommended HTTP security headers on
// every response. CSP is intentionally restrictive and assumes a JSON API.
// Tune values per environment when serving HTML or third-party widgets.
//
// References:
//   - OWASP Secure Headers Project
//   - https://infosec.mozilla.org/guidelines/web_security
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=()")
		// HSTS is harmless over HTTP (ignored) but mandatory once TLS is on.
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		// JSON API → no inline scripts, no framing, no plugins.
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-site")
		c.Next()
	}
}
