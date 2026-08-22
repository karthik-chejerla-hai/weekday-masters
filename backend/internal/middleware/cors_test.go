package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

const allowedOrigin = "https://rally.example"

func corsRouter() *gin.Engine {
	r := gin.New()
	r.Use(CORS(allowedOrigin))
	r.GET("/thing", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestCORS_AllowsTheConfiguredFrontendOrigin(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/thing", nil)
	req.Header.Set("Origin", allowedOrigin)
	corsRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Fatalf("expected the frontend origin to be echoed back, got %q", got)
	}
}

func TestCORS_AnswersPreflightWithoutReachingTheHandler(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodOptions, "/thing", nil)
	req.Header.Set("Origin", allowedOrigin)
	req.Header.Set("Access-Control-Request-Method", "POST")
	corsRouter().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for a preflight, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatal("expected the preflight to advertise the allowed methods")
	}
}

func TestCORS_DoesNotAuthoriseOtherOrigins(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/thing", nil)
	req.Header.Set("Origin", "https://not-the-frontend.example")
	corsRouter().ServeHTTP(w, req)

	// The browser is the thing being protected here: what matters is that the
	// response never carries an Allow-Origin for an origin we did not configure.
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no Allow-Origin for a foreign origin, got %q", got)
	}
}
