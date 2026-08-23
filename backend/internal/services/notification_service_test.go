package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/models"
)

func TestNotificationService_PreferencesAndTokens(t *testing.T) {
	requireDB(t)

	ns := NewNotificationService(NotificationConfig{
		FrontendURL: "https://rally.test",
	})
	user := newUser(t, "notifplayer")

	// Get default preferences
	prefs, err := ns.GetUserPreferences(user.ID)
	if err != nil {
		t.Fatalf("failed to get preferences: %v", err)
	}
	if !prefs.PushEnabled || !prefs.EmailEnabled {
		t.Fatal("expected default push and email to be enabled")
	}

	// Update preferences
	updatedPrefs, err := ns.UpdateUserPreferences(user.ID, map[string]interface{}{
		"push_enabled":  false,
		"email_enabled": true,
	})
	if err != nil {
		t.Fatalf("failed to update preferences: %v", err)
	}
	if updatedPrefs.PushEnabled {
		t.Fatal("expected push_enabled to be false")
	}

	// Register Push Tokens
	token1 := "fcm_token_sample_1"
	if err := ns.RegisterPushToken(user.ID, token1, "iPhone 15"); err != nil {
		t.Fatalf("failed to register token 1: %v", err)
	}

	// Registering same token updates last used
	if err := ns.RegisterPushToken(user.ID, token1, "iPhone 15 Pro"); err != nil {
		t.Fatalf("failed to re-register token: %v", err)
	}

	// Unregister token
	if err := ns.UnregisterPushToken(user.ID, token1); err != nil {
		t.Fatalf("failed to unregister token: %v", err)
	}
}

func TestNotificationService_SendAndHistory(t *testing.T) {
	requireDB(t)

	ns := NewNotificationService(NotificationConfig{
		FrontendURL: "https://rally.test",
	})
	user := newUser(t, "historyplayer")

	// Send notification (stores record in database)
	err := ns.SendNotification(
		context.Background(),
		user.ID,
		models.NotificationWaitlistUpdate,
		"Spot Available!",
		"You have been promoted to confirmed player.",
		map[string]string{"session_id": uuid.NewString()},
	)
	if err != nil {
		t.Fatalf("failed to send notification: %v", err)
	}

	// Retrieve history
	history, err := ns.GetUserNotifications(user.ID, 10, 0)
	if err != nil {
		t.Fatalf("failed to get notification history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 notification in history, got %d", len(history))
	}
	if history[0].Title != "Spot Available!" {
		t.Fatalf("unexpected title: %q", history[0].Title)
	}
	if history[0].ReadAt != nil {
		t.Fatal("new notification should be unread")
	}

	// Mark as read
	if err := ns.MarkNotificationRead(history[0].ID, user.ID); err != nil {
		t.Fatalf("failed to mark notification read: %v", err)
	}

	updatedHistory, err := ns.GetUserNotifications(user.ID, 10, 0)
	if err != nil || len(updatedHistory) == 0 || updatedHistory[0].ReadAt == nil {
		t.Fatal("expected notification to have read_at timestamp")
	}
}
