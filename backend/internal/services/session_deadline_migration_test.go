package services

import (
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestRSVPDeadlineMigrationNormalizesOpenFutureWindowsOnly(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")
	cutoff := time.Date(2030, 1, 1, 12, 0, 0, 0, utils.SydneyLocation)
	type testCase struct {
		name     string
		day      time.Time
		deadline time.Time
		status   models.SessionStatus
		changes  bool
	}
	cases := []testCase{
		{"open custom", cutoff.AddDate(0, 0, 10), cutoff.Add(time.Hour), models.SessionStatusOpen, true},
		{"short notice", cutoff.AddDate(0, 0, 1), cutoff.Add(6 * time.Hour), models.SessionStatusOpen, true},
		{"already closed", cutoff.AddDate(0, 0, 10), cutoff.Add(-time.Hour), models.SessionStatusOpen, false},
		{"deadline equals cutoff", cutoff.AddDate(0, 0, 10), cutoff, models.SessionStatusOpen, false},
		{"closed status", cutoff.AddDate(0, 0, 10), cutoff.Add(time.Hour), models.SessionStatusClosed, false},
		{"cancelled", cutoff.AddDate(0, 0, 10), cutoff.Add(time.Hour), models.SessionStatusCancelled, false},
		{"historical", cutoff.AddDate(0, 0, -10), cutoff.Add(time.Hour), models.SessionStatusOpen, false},
	}
	sessions := make([]models.Session, len(cases))
	for i, c := range cases {
		session, err := ss.CreateSession(CreateSessionInput{Title: c.name, SessionDate: c.day, StartTime: "18:00", EndTime: "20:00", Courts: 1, CreatedBy: creator.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err := database.DB.Model(session).Updates(map[string]any{"rsvp_deadline": c.deadline, "status": c.status}).Error; err != nil {
			t.Fatal(err)
		}
		sessions[i] = *session
	}
	if err := database.DB.Transaction(func(tx *gorm.DB) error { return database.ApplyRSVPDeadline48hWithin(tx, cutoff) }); err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		var stored models.Session
		if err := database.DB.First(&stored, "id = ?", sessions[i].ID).Error; err != nil {
			t.Fatal(err)
		}
		expected := c.deadline
		if c.changes {
			expected = utils.CalculateRSVPDeadline(*stored.StartsAt)
		}
		if !stored.RSVPDeadline.Equal(expected) {
			t.Fatalf("%s: expected %v, got %v", c.name, expected, stored.RSVPDeadline)
		}
	}
}
func TestRSVPDeadlineMigrationUsesElapsedHoursAcrossDST(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")
	for _, date := range []string{"2026-10-05", "2026-04-06"} {
		day, err := utils.ParseDateInSydney(date)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ss.CreateSession(CreateSessionInput{Title: "DST", SessionDate: day, StartTime: "18:00", EndTime: "20:00", Courts: 1, CreatedBy: creator.ID})
		if err != nil {
			t.Fatal(err)
		}
		cutoff := session.StartsAt.Add(-7 * 24 * time.Hour)
		custom := cutoff.Add(time.Hour)
		if err := database.DB.Model(session).UpdateColumn("rsvp_deadline", custom).Error; err != nil {
			t.Fatal(err)
		}
		if err := database.DB.Transaction(func(tx *gorm.DB) error { return database.ApplyRSVPDeadline48hWithin(tx, cutoff) }); err != nil {
			t.Fatal(err)
		}
		var stored models.Session
		if err := database.DB.First(&stored, "id = ?", session.ID).Error; err != nil {
			t.Fatal(err)
		}
		if stored.StartsAt.Sub(stored.RSVPDeadline) != 48*time.Hour {
			t.Fatalf("SQL deadline drifted over DST: %s", date)
		}
	}
}
func TestRSVPDeadlineMigrationVersionDoesNotRerunBusinessNormalization(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")
	calls := 0
	apply := func(tx *gorm.DB, cutoff time.Time) error {
		calls++
		return database.ApplyRSVPDeadline48hWithin(tx, cutoff)
	}
	if err := database.RunNotificationMigration(database.DB, database.RSVP48HourMigrationVersion, apply); err != nil {
		t.Fatal(err)
	}
	session := newSession(t, ss, creator.ID, 1)
	custom := utils.NowInSydney().Add(time.Hour).Truncate(time.Second)
	if err := database.DB.Model(session).UpdateColumn("rsvp_deadline", custom).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.RunNotificationMigration(database.DB, database.RSVP48HourMigrationVersion, apply); err != nil {
		t.Fatal(err)
	}
	var stored models.Session
	if err := database.DB.First(&stored, "id = ?", session.ID).Error; err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !stored.RSVPDeadline.Equal(custom) {
		t.Fatal("repeat deployment reran deadline/stage cutover")
	}
}
