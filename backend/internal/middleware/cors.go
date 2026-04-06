package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS(frontendURL string) gin.HandlerFunc {
	config := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}

	if frontendURL == "*" {
		// For preview deployments: allow any origin dynamically
		config.AllowOriginFunc = func(origin string) bool {
			return true
		}
	} else {
		config.AllowOrigins = []string{frontendURL}
	}

	return cors.New(config)
}
