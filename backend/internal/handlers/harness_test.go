package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
	"github.com/weekday-masters/backend/internal/testsupport"
)

// The handlers are thin, but "thin" is exactly where status codes and error
// shapes live, and those are what the frontend and the OpenAPI spec are written
// against. These tests mount the real handlers over the real services and a real
// database, and assert on the HTTP surface.
//
// Token validation is deliberately not exercised here — middleware owns that,
// and token_test.go covers it. The harness injects an already-authenticated user
// the same way AuthMiddleware would, so a handler bug cannot hide behind a
// mocked-out auth layer.

type harness struct {
	t       *testing.T
	router  *gin.Engine
	current *models.User
}

// newHarness gives each test an empty schema and a router with every route
// mounted. Pass nil to `as` for an unauthenticated request.
func newHarness(t *testing.T) *harness {
	t.Helper()
	testsupport.RequireDB(t)

	h := &harness{t: t}

	userService := services.NewUserService("")
	sessionService := services.NewSessionService()
	rsvpService := services.NewRSVPService(nil)
	notificationService := services.NewNotificationService(services.NotificationConfig{})
	ledgerService := services.NewLedgerService()
	settlementService := services.NewSettlementService(ledgerService)

	userHandler := NewUserHandler(userService)
	sessionHandler := NewSessionHandler(sessionService, rsvpService)
	rsvpHandler := NewRSVPHandler(rsvpService)
	adminHandler := NewAdminHandler(userService, sessionService, rsvpService)
	notificationHandler := NewNotificationHandler(notificationService)
	ledgerHandler := NewLedgerHandler(ledgerService)
	settlementHandler := NewSettlementHandler(settlementService)
	// An empty Auth0 domain: first-login registration needs a live tenant, so the
	// tests here cover the returning-user path and the fetch-failure branch.
	authHandler := NewAuthHandler(userService, services.NewAuth0Service(""))

	r := gin.New()
	r.Use(gin.Recovery())

	// Stands in for AuthMiddleware: puts the current user on the context.
	r.Use(func(c *gin.Context) {
		if h.current != nil {
			c.Set("user", h.current)
			c.Set("userID", h.current.ID)
			c.Set("auth0ID", h.current.Auth0ID)
			c.Set("accessToken", "test-access-token")
		}
		c.Next()
	})

	// The route table mirrors cmd/server/main.go. Paths are part of the contract
	// the frontend and the OpenAPI spec are written against, so the harness must
	// not invent its own.
	api := r.Group("/api")
	{
		api.POST("/auth/callback", authHandler.Callback)

		api.GET("/openapi", RedirectOpenAPI)
		api.GET("/openapi/index.html", ServeOpenAPIIndex)
		api.GET("/openapi/openapi.yaml", ServeOpenAPISpec)

		api.GET("/users/me", userHandler.GetMe)
		api.PUT("/users/me", userHandler.UpdateMe)

		api.GET("/users/me/notifications", notificationHandler.GetPreferences)
		api.PUT("/users/me/notifications", notificationHandler.UpdatePreferences)
		api.POST("/users/me/push-tokens", notificationHandler.RegisterPushToken)
		api.DELETE("/users/me/push-tokens", notificationHandler.UnregisterPushToken)
		api.GET("/users/me/notifications/history", notificationHandler.GetNotificationHistory)
		api.POST("/notifications/:id/read", notificationHandler.MarkNotificationRead)

		api.GET("/users", userHandler.ListMembers)

		api.GET("/sessions", sessionHandler.ListSessions)
		api.GET("/sessions/cancelled", sessionHandler.ListCancelledSessions)
		api.GET("/sessions/:id", sessionHandler.GetSession)

		api.POST("/sessions/:id/rsvp", rsvpHandler.CreateRSVP)
		api.PUT("/sessions/:id/rsvp", rsvpHandler.UpdateRSVP)
		api.DELETE("/sessions/:id/rsvp", rsvpHandler.DeleteRSVP)
		api.GET("/sessions/:id/rsvp/me", rsvpHandler.GetMyRSVP)

		api.GET("/accounts", ledgerHandler.ListBalances)
		api.GET("/accounts/me", ledgerHandler.GetMyBalance)
		api.GET("/accounts/me/entries", ledgerHandler.GetMyEntries)
		api.GET("/sessions/history", settlementHandler.ListSessionHistory)
		api.GET("/sessions/:id/settlement", settlementHandler.GetSessionSettlement)
	}

	admin := r.Group("/api/admin")
	{
		admin.GET("/join-requests", adminHandler.ListJoinRequests)
		admin.POST("/join-requests/:id/approve", adminHandler.ApproveJoinRequest)
		admin.POST("/join-requests/:id/reject", adminHandler.RejectJoinRequest)
		admin.PUT("/users/:id/role", adminHandler.UpdateUserRole)
		admin.POST("/sessions", adminHandler.CreateSession)
		admin.PUT("/sessions/:id", adminHandler.UpdateSession)
		admin.DELETE("/sessions/:id", adminHandler.DeleteSession)
		admin.POST("/sessions/:id/cancel", adminHandler.CancelSession)
		admin.POST("/sessions/:id/rsvp/:userId", adminHandler.AddPlayerRSVP)
		admin.PUT("/club", adminHandler.UpdateClub)
		admin.POST("/announcements", notificationHandler.SendAnnouncement)

		admin.POST("/transactions/topup", ledgerHandler.RecordTopup)
		admin.POST("/transactions/withdrawal", ledgerHandler.RecordWithdrawal)
		admin.POST("/transactions/court-credit", ledgerHandler.RecordCourtCredit)
		admin.POST("/transactions/shuttle-purchase", ledgerHandler.RecordShuttlePurchase)
		admin.POST("/transactions/opening-balances", ledgerHandler.RecordOpeningBalances)
		admin.POST("/transactions/:id/reverse", ledgerHandler.ReverseTransaction)
		admin.POST("/sessions/:id/settlement/preview", settlementHandler.PreviewSettlement)
		admin.POST("/sessions/:id/settle", settlementHandler.SettleSession)
		admin.POST("/settlements/:id/reverse", settlementHandler.ReverseSettlement)
		admin.GET("/position", ledgerHandler.GetPosition)
		admin.GET("/position/integrity", ledgerHandler.GetIntegrity)
		// NOTE: AdminHandler.GetClub is deliberately absent — main.go registers
		// only PUT /admin/club, so the GET handler is unreachable in the running
		// server. Mounting it here would manufacture coverage for dead code.
	}

	h.router = r
	return h
}

// as sets the user the next requests are made as. nil means unauthenticated.
func (h *harness) as(user *models.User) *harness {
	h.current = user
	return h
}

type response struct {
	t    *testing.T
	Code int
	Body []byte
}

func (h *harness) request(method, path string, body any) *response {
	h.t.Helper()

	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("failed to marshal request body: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, path, reader)
	if err != nil {
		h.t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)

	return &response{t: h.t, Code: w.Code, Body: w.Body.Bytes()}
}

func (h *harness) get(path string) *response { return h.request(http.MethodGet, path, nil) }
func (h *harness) del(path string) *response { return h.request(http.MethodDelete, path, nil) }
func (h *harness) post(path string, body any) *response {
	return h.request(http.MethodPost, path, body)
}
func (h *harness) put(path string, body any) *response {
	return h.request(http.MethodPut, path, body)
}

// expect asserts the status code, reporting the body when it does not match so
// a failure says why rather than just which number came back.
func (r *response) expect(code int) *response {
	r.t.Helper()
	if r.Code != code {
		r.t.Fatalf("expected status %d, got %d: %s", code, r.Code, r.Body)
	}
	return r
}

// decode unmarshals the response body into v.
func (r *response) decode(v any) {
	r.t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		r.t.Fatalf("failed to decode response %s: %v", r.Body, err)
	}
}

// errorMessage pulls the `error` field out of a failure response.
func (r *response) errorMessage() string {
	r.t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	r.decode(&body)
	return body.Error
}

// --- fixtures -------------------------------------------------------------

func makeUser(t *testing.T, role models.UserRole, status models.MembershipStatus) *models.User {
	t.Helper()

	id := uuid.NewString()
	user := models.User{
		Auth0ID:          "auth0|" + id,
		Email:            id + "@example.com",
		Name:             "Test " + string(role),
		Role:             role,
		MembershipStatus: status,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return &user
}

func makePlayer(t *testing.T) *models.User {
	return makeUser(t, models.RolePlayer, models.MembershipApproved)
}

func makeAdmin(t *testing.T) *models.User {
	return makeUser(t, models.RoleAdmin, models.MembershipApproved)
}

func makePending(t *testing.T) *models.User {
	return makeUser(t, models.RolePending, models.MembershipPending)
}

// makeSession creates an open session far enough ahead that RSVPs are still open.
func makeSession(t *testing.T, creator uuid.UUID, courts int) *models.Session {
	t.Helper()

	session, err := services.NewSessionService().CreateSession(services.CreateSessionInput{
		Title:       "Test Session",
		SessionDate: time.Now().AddDate(0, 0, 10),
		StartTime:   "18:00",
		EndTime:     "20:00",
		Courts:      courts,
		CreatedBy:   creator,
	})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	return session
}

// eventually polls probe until it reports success or the deadline passes, and
// returns its last value either way. Used for the fire-and-forget notification
// fan-out, which returns before its rows are written.
func eventually[T any](t *testing.T, probe func() (T, bool)) T {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for {
		value, done := probe()
		if done || time.Now().After(deadline) {
			return value
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// countUsersWithAuth0ID counts user rows for a subject, for assertions about
// registration side effects.
func (h *harness) countUsersWithAuth0ID(auth0ID string, into *int64) error {
	return database.DB.Model(&models.User{}).Where("auth0_id = ?", auth0ID).Count(into).Error
}
