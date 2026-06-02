package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/malekradhouane/magic/middleware"
	"github.com/malekradhouane/magic/pkg/sse"
)

// NotificationHandler streams real-time admin notifications over SSE.
type NotificationHandler struct {
	hub  *sse.Hub
	auth gin.HandlerFunc
}

// NewNotificationHandler constructs a NotificationHandler.
func NewNotificationHandler(hub *sse.Hub, auth gin.HandlerFunc) *NotificationHandler {
	return &NotificationHandler{hub: hub, auth: auth}
}

// SetupRoutes registers the SSE endpoint.
//
//	GET /admin/notifications/stream   (admin, SSE)
//
// EventSource cannot send custom headers, so the bearer token is accepted via
// the ?token= query param and promoted to an Authorization header before the
// standard auth + admin checks run.
func (h *NotificationHandler) SetupRoutes(g *gin.RouterGroup) {
	grp := g.Group("")
	grp.Use(sseQueryToken())
	grp.Use(h.auth)
	grp.Use(middleware.RequireAdmin())
	grp.GET("/admin/notifications/stream", h.Stream)
}

// sseQueryToken copies a ?token= query param into the Authorization header so
// the existing JWT middleware can validate it.
func sseQueryToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			if tok := c.Query("token"); tok != "" {
				c.Request.Header.Set("Authorization", "Bearer "+tok)
			}
		}
		c.Next()
	}
}

// Stream keeps an SSE connection open and forwards every hub message to the client.
func (h *NotificationHandler) Stream(c *gin.Context) {
	flusher, ok := c.Writer.(interface{ Flush() })
	if !ok {
		c.String(500, "streaming unsupported")
		return
	}

	id, ch := h.hub.Subscribe()
	defer h.hub.Unsubscribe(id)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	// Disable proxy buffering (nginx) so events are delivered immediately.
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(200)

	fmt.Fprint(c.Writer, ": connected\n\n")
	flusher.Flush()

	keepAlive := time.NewTicker(25 * time.Second)
	defer keepAlive.Stop()

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, open := <-ch:
			if !open {
				return
			}
			if msg.Event != "" {
				fmt.Fprintf(c.Writer, "event: %s\n", msg.Event)
			}
			fmt.Fprintf(c.Writer, "data: %s\n\n", msg.Data)
			flusher.Flush()
		case <-keepAlive.C:
			fmt.Fprint(c.Writer, ": ping\n\n")
			flusher.Flush()
		}
	}
}
