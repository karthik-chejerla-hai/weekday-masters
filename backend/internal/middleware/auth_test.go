package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/models"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRequireApproved_ApprovedUser(t *testing.T) {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{
			ID:               uuid.New(),
			MembershipStatus: models.MembershipApproved,
		})
		c.Next()
	})
	r.Use(RequireApproved())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	c.Request, _ = http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, c.Request)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRequireApproved_PendingUser(t *testing.T) {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{
			ID:               uuid.New(),
			MembershipStatus: models.MembershipPending,
		})
		c.Next()
	})
	r.Use(RequireApproved())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	c.Request, _ = http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, c.Request)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d for pending user, got %d", http.StatusForbidden, w.Code)
	}
}

func TestRequireApproved_MissingUser(t *testing.T) {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(RequireApproved())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	c.Request, _ = http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, c.Request)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d when user not in context, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestRequireAdmin_AdminUser(t *testing.T) {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{
			ID:   uuid.New(),
			Role: models.RoleAdmin,
		})
		c.Next()
	})
	r.Use(RequireAdmin())
	r.GET("/admin-only", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	c.Request, _ = http.NewRequest(http.MethodGet, "/admin-only", nil)
	r.ServeHTTP(w, c.Request)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d for admin, got %d", http.StatusOK, w.Code)
	}
}

func TestRequireAdmin_NonAdminUser(t *testing.T) {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{
			ID:   uuid.New(),
			Role: models.RolePlayer,
		})
		c.Next()
	})
	r.Use(RequireAdmin())
	r.GET("/admin-only", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	c.Request, _ = http.NewRequest(http.MethodGet, "/admin-only", nil)
	r.ServeHTTP(w, c.Request)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d for player accessing admin route, got %d", http.StatusForbidden, w.Code)
	}
}

func TestGetUserFromContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Missing user
	_, err := GetUserFromContext(c)
	if err == nil {
		t.Error("expected error when user missing from context, got nil")
	}

	// Valid user
	expectedUser := &models.User{ID: uuid.New(), Name: "Test Player"}
	c.Set("user", expectedUser)
	user, err := GetUserFromContext(c)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if user.ID != expectedUser.ID {
		t.Errorf("expected user ID %v, got %v", expectedUser.ID, user.ID)
	}
}

func TestGetAuth0IDAndTokenFromContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Unset
	if _, err := GetAuth0IDFromContext(c); err == nil {
		t.Error("expected error when auth0ID is unset")
	}
	if _, err := GetAccessTokenFromContext(c); err == nil {
		t.Error("expected error when accessToken is unset")
	}

	// Set
	c.Set("auth0ID", "auth0|123456")
	c.Set("accessToken", "eySampleToken")

	auth0ID, err := GetAuth0IDFromContext(c)
	if err != nil || auth0ID != "auth0|123456" {
		t.Errorf("unexpected auth0ID: %s (err: %v)", auth0ID, err)
	}

	token, err := GetAccessTokenFromContext(c)
	if err != nil || token != "eySampleToken" {
		t.Errorf("unexpected token: %s (err: %v)", token, err)
	}
}

func TestWithUser(t *testing.T) {
	user := &models.User{ID: uuid.New(), Name: "Context User"}
	ctx := WithUser(context.Background(), user)

	val := ctx.Value(UserContextKey)
	u, ok := val.(*models.User)
	if !ok || u.ID != user.ID {
		t.Errorf("expected user in context %v, got %v", user, val)
	}
}
