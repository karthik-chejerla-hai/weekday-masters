package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMaxPlayersForCourts(t *testing.T) {
	tests := []struct {
		courts int
		want   int
	}{
		{courts: 1, want: 6},
		{courts: 2, want: 10},
		{courts: 3, want: 16},
		{courts: 4, want: 6},  // fallback
		{courts: 0, want: 6},  // fallback
	}

	for _, tt := range tests {
		if got := MaxPlayersForCourts(tt.courts); got != tt.want {
			t.Errorf("MaxPlayersForCourts(%d) = %d, want %d", tt.courts, got, tt.want)
		}
	}
}

func TestSession_IsRSVPOpen(t *testing.T) {
	futureSession := Session{
		RSVPDeadline: time.Now().Add(24 * time.Hour),
	}
	if !futureSession.IsRSVPOpen() {
		t.Error("expected IsRSVPOpen to be true for future deadline")
	}

	pastSession := Session{
		RSVPDeadline: time.Now().Add(-1 * time.Hour),
	}
	if pastSession.IsRSVPOpen() {
		t.Error("expected IsRSVPOpen to be false for past deadline")
	}
}

func TestUser_IsApprovedAndIsAdmin(t *testing.T) {
	pendingUser := User{
		Role:             RolePending,
		MembershipStatus: MembershipPending,
	}
	if pendingUser.IsApproved() {
		t.Error("pending user should not be approved")
	}
	if pendingUser.IsAdmin() {
		t.Error("pending user should not be admin")
	}

	playerUser := User{
		Role:             RolePlayer,
		MembershipStatus: MembershipApproved,
	}
	if !playerUser.IsApproved() {
		t.Error("player with approved status should be approved")
	}
	if playerUser.IsAdmin() {
		t.Error("player should not be admin")
	}

	adminUser := User{
		Role:             RoleAdmin,
		MembershipStatus: MembershipApproved,
	}
	if !adminUser.IsApproved() {
		t.Error("admin with approved status should be approved")
	}
	if !adminUser.IsAdmin() {
		t.Error("admin role user should be admin")
	}
}

func TestUserNotificationPreferences_Toggles(t *testing.T) {
	prefs := UserNotificationPreferences{
		PushEnabled:            true,
		PushSessionReminders:   true,
		PushRSVPDeadlines:      false,
		PushWaitlistUpdates:    true,
		PushAdminAnnouncements: false,

		EmailEnabled:            true,
		EmailSessionReminders:   false,
		EmailRSVPDeadlines:      true,
		EmailWaitlistUpdates:    false,
		EmailAdminAnnouncements: true,
	}

	// Push checks
	if !prefs.IsPushEnabledForType(NotificationSessionReminder) {
		t.Error("push session reminders should be enabled")
	}
	if prefs.IsPushEnabledForType(NotificationRSVPDeadline) {
		t.Error("push rsvp deadlines should be disabled")
	}
	if !prefs.IsPushEnabledForType(NotificationWaitlistUpdate) {
		t.Error("push waitlist updates should be enabled")
	}
	if prefs.IsPushEnabledForType(NotificationAdminAnnouncement) {
		t.Error("push admin announcements should be disabled")
	}

	// Email checks
	if prefs.IsEmailEnabledForType(NotificationSessionReminder) {
		t.Error("email session reminders should be disabled")
	}
	if !prefs.IsEmailEnabledForType(NotificationRSVPDeadline) {
		t.Error("email rsvp deadlines should be enabled")
	}
	if prefs.IsEmailEnabledForType(NotificationWaitlistUpdate) {
		t.Error("email waitlist updates should be disabled")
	}
	if !prefs.IsEmailEnabledForType(NotificationAdminAnnouncement) {
		t.Error("email admin announcements should be enabled")
	}

	// Master switches
	prefs.PushEnabled = false
	if prefs.IsPushEnabledForType(NotificationSessionReminder) {
		t.Error("when PushEnabled is false, all push types should return false")
	}

	prefs.EmailEnabled = false
	if prefs.IsEmailEnabledForType(NotificationRSVPDeadline) {
		t.Error("when EmailEnabled is false, all email types should return false")
	}

	// Unknown type
	if prefs.IsPushEnabledForType("unknown_type") {
		t.Error("unknown type should return false")
	}
	if prefs.IsEmailEnabledForType("unknown_type") {
		t.Error("unknown type should return false")
	}
}

func TestModelBeforeCreateHooks(t *testing.T) {
	user := &User{}
	_ = user.BeforeCreate(nil)
	if user.ID == uuid.Nil {
		t.Error("expected user ID to be initialized")
	}

	session := &Session{Courts: 2}
	_ = session.BeforeCreate(nil)
	if session.ID == uuid.Nil {
		t.Error("expected session ID to be initialized")
	}
	if session.MaxPlayers != 10 {
		t.Errorf("expected session max players to be 10, got %d", session.MaxPlayers)
	}

	pref := &UserNotificationPreferences{}
	_ = pref.BeforeCreate(nil)
	if pref.ID == uuid.Nil {
		t.Error("expected pref ID to be initialized")
	}

	pushToken := &UserPushToken{}
	_ = pushToken.BeforeCreate(nil)
	if pushToken.ID == uuid.Nil {
		t.Error("expected push token ID to be initialized")
	}
	if pushToken.LastUsedAt.IsZero() {
		t.Error("expected push token LastUsedAt to be set")
	}

	notif := &Notification{}
	_ = notif.BeforeCreate(nil)
	if notif.ID == uuid.Nil {
		t.Error("expected notification ID to be initialized")
	}

	announcement := &Announcement{}
	_ = announcement.BeforeCreate(nil)
	if announcement.ID == uuid.Nil {
		t.Error("expected announcement ID to be initialized")
	}
	if announcement.SentAt.IsZero() {
		t.Error("expected announcement SentAt to be set")
	}
}
