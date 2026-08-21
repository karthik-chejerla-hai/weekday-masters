package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm/logger"
)

// These tests exercise real SQL — transaction boundaries and row locks are the whole
// point of the capacity logic, so an in-memory fake would not prove anything.
//
// Point TEST_DATABASE_URL at a scratch database to run them:
//
//	docker run -d --name rally-test-db -p 5433:5432 \
//	  -e POSTGRES_USER=badminton -e POSTGRES_PASSWORD=badminton123 \
//	  -e POSTGRES_DB=badminton_club_test postgres:16-alpine
//	TEST_DATABASE_URL="postgres://badminton:badminton123@localhost:5433/badminton_club_test?sslmode=disable" \
//	  go test ./internal/services/
//
// With the variable unset the database-backed tests skip, so `go test ./...` stays
// green on a machine with no database. NEVER point this at a real database: every
// test truncates the schema.
var dbAvailable bool

func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		log.Println("TEST_DATABASE_URL not set — skipping database-backed tests")
		os.Exit(m.Run())
	}

	if err := database.Connect(url); err != nil {
		log.Fatalf("TEST_DATABASE_URL is set but unusable: %v", err)
	}
	database.DB.Logger = logger.Default.LogMode(logger.Silent)

	if err := database.Migrate(); err != nil {
		log.Fatalf("failed to migrate test database: %v", err)
	}

	dbAvailable = true
	os.Exit(m.Run())
}

// requireDB skips the test when no scratch database is configured, and otherwise
// hands it an empty schema.
func requireDB(t *testing.T) {
	t.Helper()

	if !dbAvailable {
		t.Skip("set TEST_DATABASE_URL to run database-backed tests")
	}

	const stmt = `TRUNCATE TABLE rsvps, notifications, user_push_tokens,
		user_notification_preferences, announcements, sessions, users
		RESTART IDENTITY CASCADE`
	if err := database.DB.Exec(stmt).Error; err != nil {
		t.Fatalf("failed to reset test database: %v", err)
	}
}

// recordingNotifier captures waitlist promotion notifications.
type recordingNotifier struct {
	mu   sync.Mutex
	sent []sentNotification
}

type sentNotification struct {
	UserID uuid.UUID
	Type   models.NotificationType
	Title  string
}

func (n *recordingNotifier) SendNotification(
	_ context.Context,
	userID uuid.UUID,
	notifType models.NotificationType,
	title, _ string,
	_ map[string]string,
) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sent = append(n.sent, sentNotification{UserID: userID, Type: notifType, Title: title})
	return nil
}

func (n *recordingNotifier) notifiedUsers() []uuid.UUID {
	n.mu.Lock()
	defer n.mu.Unlock()

	ids := make([]uuid.UUID, 0, len(n.sent))
	for _, s := range n.sent {
		ids = append(ids, s.UserID)
	}
	return ids
}

func (n *recordingNotifier) reset() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sent = nil
}

// newTestServices returns the services under test plus the notifier they report to.
func newTestServices(t *testing.T) (*RSVPService, *SessionService, *recordingNotifier) {
	t.Helper()
	requireDB(t)

	notifier := &recordingNotifier{}
	return NewRSVPService(notifier), NewSessionService(), notifier
}

// newUser creates an approved player.
func newUser(t *testing.T, label string) models.User {
	t.Helper()

	id := uuid.NewString()
	user := models.User{
		Auth0ID:          "auth0|" + label + "-" + id,
		Email:            label + "-" + id + "@example.com",
		Name:             label,
		Role:             models.RolePlayer,
		MembershipStatus: models.MembershipApproved,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user %s: %v", label, err)
	}
	return user
}

// newUsers creates n approved players.
func newUsers(t *testing.T, n int) []models.User {
	t.Helper()

	users := make([]models.User, 0, n)
	for i := 0; i < n; i++ {
		users = append(users, newUser(t, fmt.Sprintf("player%02d", i)))
	}
	return users
}

// newSession creates an open session far enough ahead that the RSVP deadline is future.
func newSession(t *testing.T, ss *SessionService, creator uuid.UUID, courts int) *models.Session {
	t.Helper()

	session, err := ss.CreateSession(CreateSessionInput{
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

// rsvpIn is the common case: a player says they are coming.
func rsvpIn(t *testing.T, rs *RSVPService, sessionID, userID uuid.UUID) *models.RSVP {
	t.Helper()

	rsvp, err := rs.CreateOrUpdateRSVP(RSVPInput{
		SessionID: sessionID,
		UserID:    userID,
		Status:    models.RSVPStatusIn,
	}, false)
	if err != nil {
		t.Fatalf("rsvp failed: %v", err)
	}
	return rsvp
}

// statusOf reads a player's stored RSVP.
func statusOf(t *testing.T, rs *RSVPService, sessionID, userID uuid.UUID) *models.RSVP {
	t.Helper()

	rsvp, err := rs.GetUserRSVPForSession(sessionID, userID)
	if err != nil {
		t.Fatalf("failed to read rsvp: %v", err)
	}
	return rsvp
}

// summaryOf reads the session's RSVP counts.
func summaryOf(t *testing.T, rs *RSVPService, sessionID uuid.UUID) *RSVPSummary {
	t.Helper()

	summary, err := rs.GetRSVPSummary(sessionID)
	if err != nil {
		t.Fatalf("failed to read summary: %v", err)
	}
	return summary
}
