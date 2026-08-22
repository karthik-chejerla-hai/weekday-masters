package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	return r
}

func TestSessionHandler_ListSessions_ErrorHandling(t *testing.T) {
	sh := NewSessionHandler(services.NewSessionService(), services.NewRSVPService(nil))
	r := setupTestRouter()
	r.GET("/api/sessions", sh.ListSessions)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/sessions", nil)
	r.ServeHTTP(w, req)

	// Since DB is not connected in unit test mode, should handle gracefully (e.g. 500 status)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status: %d", w.Code)
	}
}

func TestSessionHandler_GetSession_InvalidUUID(t *testing.T) {
	sh := NewSessionHandler(services.NewSessionService(), services.NewRSVPService(nil))
	r := setupTestRouter()
	r.GET("/api/sessions/:id", sh.GetSession)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/sessions/not-a-valid-uuid", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid UUID param, got %d", w.Code)
	}
}

func TestRSVPHandler_CreateRSVP_Validation(t *testing.T) {
	rh := NewRSVPHandler(services.NewRSVPService(nil))
	r := setupTestRouter()

	// Inject authenticated user into context
	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{ID: uuid.New(), Role: models.RolePlayer})
		c.Next()
	})
	r.POST("/api/sessions/:id/rsvp", rh.CreateRSVP)

	// Test invalid status
	invalidBody, _ := json.Marshal(map[string]string{"status": "invalid_choice"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/sessions/"+uuid.NewString()+"/rsvp", bytes.NewReader(invalidBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid status, got %d", w.Code)
	}
}

func TestUserHandler_UpdateProfile_Validation(t *testing.T) {
	uh := NewUserHandler(services.NewUserService(""))
	r := setupTestRouter()

	// Without user in context
	r.PUT("/api/users/me", uh.UpdateMe)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/users/me", bytes.NewReader([]byte(`{"phone_number": "123"}`)))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized when user not in context, got %d", w.Code)
	}
}
