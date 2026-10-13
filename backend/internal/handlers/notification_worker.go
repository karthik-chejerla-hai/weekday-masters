package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/api/idtoken"
)

// WorkerTokenValidator is injectable only for signature-validation tests.
type WorkerTokenValidator func(context.Context, string, string) (*idtoken.Payload, error)

// NewNotificationWorkerRouter is a separate private router, never mounted on the
// public API. Cloud Run IAM and the exact scheduler identity both gate requests.
func NewNotificationWorkerRouter(audience, schedulerEmail string, validate WorkerTokenValidator, run func(context.Context) (any, error)) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.POST("/internal/notifications/run", func(c *gin.Context) {
		auth := strings.Fields(c.GetHeader("Authorization"))
		if audience == "" || schedulerEmail == "" || len(auth) != 2 || auth[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Scheduler authentication required."})
			return
		}
		payload, err := validate(c.Request.Context(), auth[1], audience)
		if err != nil || payload == nil || payload.Audience != audience || payload.Expires <= time.Now().Unix() ||
			(payload.Issuer != "https://accounts.google.com" && payload.Issuer != "accounts.google.com") ||
			payload.Claims["email"] != schedulerEmail || payload.Claims["email_verified"] != true {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Scheduler authentication required."})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 50*time.Second)
		defer cancel()
		result, err := run(ctx)
		if err != nil {
			// Error details can contain provider/DB data; log safe run counters separately.
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Notification worker run failed."})
			return
		}
		c.JSON(http.StatusOK, result)
	})
	return router
}
