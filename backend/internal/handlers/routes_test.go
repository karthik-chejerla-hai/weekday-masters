package handlers

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// Gin refuses to register a static path segment that collides with a wildcard
// already claimed at the same position, and it does so by panicking at
// registration — which means a bad combination is not a failing request, it is a
// server that will not boot.
//
// The ledger and settlement work added /sessions/history alongside the existing
// /sessions/:id, which is exactly that shape. This registers the full set of
// session-scoped paths so the collision is caught here rather than on deploy.
func TestSessionRoutePathsDoNotCollide(t *testing.T) {
	gin.SetMode(gin.TestMode)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("route registration panicked, so the server would not start: %v", r)
		}
	}()

	noop := func(*gin.Context) {}
	r := gin.New()
	api := r.Group("/api")

	// Every session-scoped path the app registers, static and wildcard mixed.
	for _, path := range []string{
		"/sessions",
		"/sessions/cancelled",
		"/sessions/history",
		"/sessions/:id",
		"/sessions/:id/rsvp/me",
		"/sessions/:id/settlement",
	} {
		api.GET(path, noop)
	}

	for _, path := range []string{
		"/sessions/:id/rsvp",
		"/admin/sessions/:id/settlement/preview",
		"/admin/sessions/:id/settle",
		"/admin/sessions/:id/cancel",
		"/admin/sessions/:id/rsvp/:userId",
		"/admin/settlements/:id/reverse",
		"/admin/transactions/topup",
		"/admin/transactions/:id/reverse",
	} {
		api.POST(path, noop)
	}
}
