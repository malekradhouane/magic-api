package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/malekradhouane/magic/utils/httpresp"
)

// maxLoginBodyBytes caps the body we read for email extraction to avoid
// memory abuse on this hot path. Real payloads are tiny (a few hundred bytes).
const maxLoginBodyBytes = 8 * 1024

// LoginRateLimit applies two sliding-window rate limits to login-like
// endpoints: one per client IP and one per submitted email address.
// The stricter of the two wins. The request body is buffered and restored
// so downstream handlers can re-read it.
//
// If emailLimit is nil, only the IP gate is enforced.
func LoginRateLimit(ipLimit, emailLimit *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// IP gate first (cheap, always applied).
		if ipLimit != nil && !ipLimit.Allow("ip:"+c.ClientIP()) {
			httpresp.NewErrorMessage(c, http.StatusTooManyRequests,
				"too many attempts, try again later")
			c.Abort()
			return
		}

		// Optional email gate — only enforced when the body carries an email.
		if emailLimit != nil && c.Request.Body != nil {
			// Read one byte more than the limit so we can detect oversize bodies
			// without silently truncating them (which would yield a confusing
			// 400 downstream).
			limited := io.LimitReader(c.Request.Body, maxLoginBodyBytes+1)
			body, err := io.ReadAll(limited)
			if err != nil {
				httpresp.NewErrorMessage(c, http.StatusBadRequest, "failed to read request body")
				c.Abort()
				return
			}
			if len(body) > maxLoginBodyBytes {
				httpresp.NewErrorMessage(c, http.StatusRequestEntityTooLarge,
					"login request body is too large")
				c.Abort()
				return
			}
			if len(body) > 0 {
				// Restore body so the next handler can decode it.
				c.Request.Body = io.NopCloser(bytes.NewReader(body))

				var creds struct {
					Email string `json:"email"`
				}
				if json.Unmarshal(body, &creds) == nil {
					email := strings.ToLower(strings.TrimSpace(creds.Email))
					if email != "" && !emailLimit.Allow("email:"+email) {
						httpresp.NewErrorMessage(c, http.StatusTooManyRequests,
							"too many attempts for this account, try again later")
						c.Abort()
						return
					}
				}
			}
		}
		c.Next()
	}
}
