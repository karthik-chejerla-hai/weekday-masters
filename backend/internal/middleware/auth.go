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

// extractBearerToken pulls the raw token out of the Authorization header.
func extractBearerToken(c *gin.Context) (string, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return "", errors.New("Authorization header required")
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return "", errors.New("Bearer token required")
	}

	return tokenString, nil
}

// ValidateToken verifies an Auth0 access token's signature, audience and issuer,
// returning its claims. It performs no database access.
func ValidateToken(config Auth0Config, tokenString string) (jwt.MapClaims, error) {
	// Parse token without validation first to get the kid
	unverifiedToken, _, err := jwt.NewParser().ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, errors.New("Invalid token format")
	}

	kid, ok := unverifiedToken.Header["kid"].(string)
	if !ok {
		return nil, errors.New("Token missing key ID")
	}

	jwks, err := getJWKS(config.Domain)
	if err != nil {
		// Log the actual error for debugging; don't leak it to the caller.
		fmt.Printf("JWKS fetch error: %v\n", err)
		return nil, errJWKSUnavailable
	}

	certPEM, err := getKeyFromJWKS(jwks, kid)
	if err != nil {
		return nil, errors.New("Unable to find key")
	}

	cert, err := jwt.ParseRSAPublicKeyFromPEM([]byte(certPEM))
	if err != nil {
		return nil, errors.New("Invalid certificate")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return cert, nil
	}, jwt.WithAudience(config.Audience), jwt.WithIssuer(fmt.Sprintf("https://%s/", config.Domain)))

	if err != nil || !token.Valid {
		return nil, errors.New("Invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("Invalid claims")
	}

	return claims, nil
}

// errJWKSUnavailable signals an infrastructure failure rather than a bad token.
var errJWKSUnavailable = errors.New("Failed to verify token")

// RequireValidToken validates the Auth0 access token and stores its claims and the
// raw token on the context. Unlike AuthMiddleware it does NOT require a matching
// user row, so it can guard registration endpoints where the row does not exist yet.
func RequireValidToken(config Auth0Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := extractBearerToken(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		claims, err := ValidateToken(config, tokenString)
		if err != nil {
			status := http.StatusUnauthorized
			if errors.Is(err, errJWKSUnavailable) {
				status = http.StatusInternalServerError
			}
			c.JSON(status, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		sub, _ := claims["sub"].(string)
		if sub == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token missing subject"})
			c.Abort()
			return
		}

		c.Set("claims", claims)
		c.Set("auth0ID", sub)
		c.Set("accessToken", tokenString)

		c.Next()
	}
}

// AuthMiddleware validates JWT tokens from Auth0 and loads the matching user.
func AuthMiddleware(config Auth0Config) gin.HandlerFunc {
	validate := RequireValidToken(config)

	return func(c *gin.Context) {
		validate(c)
		if c.IsAborted() {
			return
		}

		sub := c.GetString("auth0ID")

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

		c.Next()
	}
}

// GetAuth0IDFromContext returns the verified Auth0 subject for the request.
func GetAuth0IDFromContext(c *gin.Context) (string, error) {
	sub := c.GetString("auth0ID")
	if sub == "" {
		return "", errors.New("no verified subject on request")
	}
	return sub, nil
}

// GetAccessTokenFromContext returns the raw bearer token for the request.
func GetAccessTokenFromContext(c *gin.Context) (string, error) {
	token := c.GetString("accessToken")
	if token == "" {
		return "", errors.New("no access token on request")
	}
	return token, nil
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
