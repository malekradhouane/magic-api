package gatekeeper

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"github.com/malekradhouane/magic/pkg/gatekeeper/oidc"
)

type GatekeeperContract interface {
	Application() ApplicationContract

	Close(ctx context.Context) error
}

type ApplicationContract interface {
	Authorize(obj string, act string) gin.HandlerFunc
	isValidURL(urlStr string) bool
	LoginHandler(c *gin.Context)
	CallbackHandler(c *gin.Context)

	createSession(c *gin.Context, userInfo oidc.UserInfo, token *oauth2.Token) error
}

// SessionData represents the data stored in a session
type SessionData struct {
	UserInfo     oidc.UserInfo `json:"user_info"`
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token,omitempty"`
	TokenExpiry  time.Time     `json:"token_expiry"`
	CreatedAt    time.Time     `json:"created_at"`
	LastUsedAt   time.Time     `json:"last_used_at"`
	UserAgent    string        `json:"user_agent"`
	ClientIP     string        `json:"client_ip"`
}
