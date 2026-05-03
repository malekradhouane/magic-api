package gatekeeper

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/malekradhouane/magic/pkg/assertor"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/utils/httpresp"
)

// NewControllerHttpGinParams contains the dependencies to wire the HTTP routes
type NewControllerHttpGinParams struct {
	GinRouter    *gin.Engine
	App          ApplicationContract
	SessionStore IdentityStore
	Verifier     *Verifier
}

// ctrlHttpGin wires the HTTP routes for Gatekeeper
type ctrlHttpGin struct {
	router       *gin.Engine
	app          ApplicationContract
	sessionStore IdentityStore
	verifier     *Verifier
}

// NewControllerHttpGin registers OIDC routes and a session introspection endpoint under /api
func NewControllerHttpGin(params NewControllerHttpGinParams) (*ctrlHttpGin, error) {
	v := assertor.New()
	v.Assert(params.GinRouter != nil, "router is missing")
	v.Assert(params.App != nil, "application is missing")
	if err := v.Validate(); err != nil {
		return nil, err
	}

	c := &ctrlHttpGin{
		router:       params.GinRouter,
		app:          params.App,
		sessionStore: params.SessionStore,
		verifier:     params.Verifier,
	}

	api := params.GinRouter.Group("/api")
	api.GET("/login", c.app.LoginHandler)
	api.GET("/callback", c.app.CallbackHandler)
	api.GET("/session", c.sessionHandler)

	return c, nil
}

// sessionHandler
func (c *ctrlHttpGin) sessionHandler(ctx *gin.Context) {
	if c.sessionStore != nil {
		if cookie, err := ctx.Request.Cookie("magic_session"); err == nil {
			if sess, err := c.sessionStore.Get(ctx.Request.Context(), cookie.Value); err == nil {
				httpresp.NewResult(ctx, http.StatusOK, interfaces.Identity{
					UserName: sess.UserInfo.Username,
					Role:     sess.UserInfo.Role,
				})
				return
			}
		}
	}

	if c.verifier != nil {
		if authHeader := ctx.GetHeader("Authorization"); authHeader != "" {
			if vt, err := c.verifier.VerifyToken(ctx.Request.Context(), authHeader); err == nil && vt != nil {
				httpresp.NewResult(ctx, http.StatusOK, nil)
				return
			}
		}
	}

	httpresp.NewErrorMessage(ctx, http.StatusUnauthorized, "User is not authenticated!")
}

func (c *ctrlHttpGin) Close(_ context.Context) error {
	return nil
}
