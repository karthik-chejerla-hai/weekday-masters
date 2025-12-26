package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

type Auth0Config struct {
	Domain   string
	Audience string
}

type JWKS struct {
	Keys []JSONWebKey `json:"keys"`
}

type JSONWebKey struct {
	Kty string   `json:"kty"`
	Kid string   `json:"kid"`
	Use string   `json:"use"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	X5c []string `json:"x5c"`
}

var jwksCache *JWKS
var jwksCacheTime time.Time

func getJWKS(domain string) (*JWKS, error) {
	// Cache JWKS for 1 hour
	if jwksCache != nil && time.Since(jwksCacheTime) < time.Hour {
		return jwksCache, nil
	}

	if domain == "" {
		return nil, errors.New("AUTH0_DOMAIN is not configured")
	}

	jwksURL := fmt.Sprintf("https://%s/.well-known/jwks.json", domain)
	resp, err := http.Get(jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS from %s: %w", jwksURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS response: %w", err)
	}

	jwksCache = &jwks
	jwksCacheTime = time.Now()
	return &jwks, nil
}

func getKeyFromJWKS(jwks *JWKS, kid string) (string, error) {
	for _, key := range jwks.Keys {
		if key.Kid == kid {
			if len(key.X5c) > 0 {
				return "-----BEGIN CERTIFICATE-----\n" + key.X5c[0] + "\n-----END CERTIFICATE-----", nil
			}
		}
	}
	return "", errors.New("unable to find key")
}

// AuthMiddleware validates JWT tokens from Auth0
func AuthMiddleware(config Auth0Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
			c.Abort()
			return
		}

		// Parse token without validation first to get the kid
		unverifiedToken, _, err := jwt.NewParser().ParseUnverified(tokenString, jwt.MapClaims{})
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token format"})
			c.Abort()
			return
		}

		kid, ok := unverifiedToken.Header["kid"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token missing key ID"})
			c.Abort()
			return
		}

		// Get JWKS
		jwks, err := getJWKS(config.Domain)
		if err != nil {
			// Log the actual error for debugging
			fmt.Printf("JWKS fetch error: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch JWKS", "details": err.Error()})
			c.Abort()
			return
		}

		// Get the key
		certPEM, err := getKeyFromJWKS(jwks, kid)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unable to find key"})
			c.Abort()
			return
		}

		cert, err := jwt.ParseRSAPublicKeyFromPEM([]byte(certPEM))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid certificate"})
			c.Abort()
			return
		}

		// Validate the token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return cert, nil
		}, jwt.WithAudience(config.Audience), jwt.WithIssuer(fmt.Sprintf("https://%s/", config.Domain)))

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid claims"})
			c.Abort()
			return
		}

		// Extract user info from token
		sub, _ := claims["sub"].(string)

		// Get user from database
		var user models.User
		result := database.DB.Where("auth0_id = ?", sub).First(&user)
		if result.Error != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found. Please complete registration."})
			c.Abort()
			return
		}

		// Store user in context
		c.Set("user", &user)
		c.Set("userID", user.ID)
		c.Set("auth0ID", sub)

		c.Next()
	}
}

// RequireApproved ensures the user has approved membership
func RequireApproved() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
			c.Abort()
			return
		}

		u, ok := user.(*models.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user type"})
			c.Abort()
			return
		}

		if !u.IsApproved() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Membership not approved"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAdmin ensures the user has admin role
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
			c.Abort()
			return
		}

		u, ok := user.(*models.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user type"})
			c.Abort()
			return
		}

		if !u.IsAdmin() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUserFromContext retrieves the current user from the Gin context
func GetUserFromContext(c *gin.Context) (*models.User, error) {
	user, exists := c.Get("user")
	if !exists {
		return nil, errors.New("user not found in context")
	}

	u, ok := user.(*models.User)
	if !ok {
		return nil, errors.New("invalid user type")
	}

	return u, nil
}

// ContextKey type for context keys
type ContextKey string

const UserContextKey ContextKey = "user"

// WithUser adds user to standard context
func WithUser(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}
