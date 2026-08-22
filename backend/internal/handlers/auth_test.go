package handlers

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/models"
)

// The registration endpoint is the one place a caller could try to choose their
// own identity, so these tests care less about the happy path than about where
// the subject and email are allowed to come from.

func TestCallback_SyncsAReturningUserWithoutTouchingAuth0(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	var body struct {
		User  models.User `json:"user"`
		IsNew bool        `json:"is_new"`
	}
	h.as(player).post("/api/auth/callback", map[string]string{
		"name":            "Updated Name",
		"profile_picture": "https://example.com/new.jpg",
	}).expect(http.StatusOK).decode(&body)

	if body.IsNew {
		t.Fatal("expected an existing user not to be reported as new")
	}
	if body.User.Name != "Updated Name" {
		t.Fatalf("expected the display name to be synced, got %q", body.User.Name)
	}
	if body.User.ProfilePicture != "https://example.com/new.jpg" {
		t.Fatalf("expected the picture to be synced, got %q", body.User.ProfilePicture)
	}
}

func TestCallback_TakesIdentityFromTheTokenNotTheBody(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	// A caller who puts someone else's identity in the body must not be able to
	// change their own email, subject, role or membership through it.
	var body struct {
		User models.User `json:"user"`
	}
	h.as(player).post("/api/auth/callback", map[string]any{
		"name":              "Impostor",
		"email":             "admin@weekdaymasters.com",
		"auth0_id":          "auth0|somebody-else",
		"role":              "admin",
		"membership_status": "approved",
	}).expect(http.StatusOK).decode(&body)

	if body.User.Email != player.Email {
		t.Fatalf("expected the email to be untouched, got %q", body.User.Email)
	}
	if body.User.Auth0ID != player.Auth0ID {
		t.Fatalf("expected the subject to be untouched, got %q", body.User.Auth0ID)
	}
	if body.User.Role != models.RolePlayer {
		t.Fatalf("expected the role to be untouched, got %q", body.User.Role)
	}
}

func TestCallback_AcceptsAnEmptyBody(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	// The display fields are optional; a bodyless sync must still succeed.
	h.as(player).post("/api/auth/callback", nil).expect(http.StatusOK)
}

func TestCallback_RejectsARequestWithNoVerifiedSubject(t *testing.T) {
	h := newHarness(t)

	h.as(nil).post("/api/auth/callback", map[string]string{"name": "Nobody"}).
		expect(http.StatusUnauthorized)
}

func TestCallback_FailsClosedWhenAuth0CannotBeReached(t *testing.T) {
	h := newHarness(t)

	// A valid token whose subject has no user row: registration must consult
	// Auth0 for the authoritative email, and must not invent a user when it
	// cannot. The harness wires an unconfigured Auth0 domain to force that.
	unregistered := &models.User{ID: uuid.New(), Auth0ID: "auth0|never-registered"}

	resp := h.as(unregistered).post("/api/auth/callback", map[string]string{"name": "Nobody"})
	if resp.Code != http.StatusInternalServerError && resp.Code != http.StatusBadGateway {
		t.Fatalf("expected registration to fail closed, got %d: %s", resp.Code, resp.Body)
	}

	var count int64
	if err := h.countUsersWithAuth0ID("auth0|never-registered", &count); err != nil {
		t.Fatalf("failed to count users: %v", err)
	}
	if count != 0 {
		t.Fatal("expected no user row to be created when the profile could not be verified")
	}
}

// --- OpenAPI docs ---------------------------------------------------------

func TestOpenAPIRoutes_ServeTheSpecAndItsViewer(t *testing.T) {
	h := newHarness(t)

	h.get("/api/openapi").expect(http.StatusMovedPermanently)

	index := h.get("/api/openapi/index.html").expect(http.StatusOK)
	if len(index.Body) == 0 {
		t.Fatal("expected the Swagger UI page to have content")
	}

	spec := h.get("/api/openapi/openapi.yaml").expect(http.StatusOK)
	if len(spec.Body) == 0 {
		t.Fatal("expected the OpenAPI spec to have content")
	}
}
