package handler

import (
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// auditLog emits a structured log entry tagged "audit" with the caller's
// identity, source IP and an action name. It is meant for security-relevant
// events (privilege change, admin creation, …) so they can be filtered out
// of regular application logs and shipped to a dedicated sink.
//
// We deliberately never log credentials, tokens or full request bodies.
// Callers should keep `fields` to small, structured key/values.
func auditLog(c *gin.Context, action string, fields map[string]any) {
	claims := jwt.ExtractClaims(c)
	actorID, _ := claims["id"].(string)
	actorRole, _ := claims["role"].(string)

	entry := logrus.WithFields(logrus.Fields{
		"audit":      true,
		"action":     action,
		"actor_id":   actorID,
		"actor_role": actorRole,
		"client_ip":  c.ClientIP(),
		"method":     c.Request.Method,
		"path":       c.Request.URL.Path,
	})
	for k, v := range fields {
		entry = entry.WithField(k, v)
	}
	entry.Info("audit event")
}
