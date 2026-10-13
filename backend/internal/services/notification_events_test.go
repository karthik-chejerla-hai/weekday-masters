package services

import (
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/models"
	"testing"
)

func TestNotificationDestinations(t *testing.T) {
	id := uuid.New().String()
	for _, test := range []struct {
		data map[string]string
		want string
	}{
		{map[string]string{"type": "session_charge", "session_id": id}, "/sessions/" + id + "/settlement"},
		{map[string]string{"type": "rsvp_deadline", "session_id": id}, "/sessions/" + id},
		{map[string]string{"type": "balance_low"}, "/money"},
		{map[string]string{"type": "admin_announcement", "notification_id": id}, "/notifications/" + id},
		{map[string]string{"destination_path": "https://evil.test", "session_id": "bad"}, "/notifications"},
	} {
		if got := NotificationDestination(test.data); got != test.want {
			t.Errorf("got %s want %s", got, test.want)
		}
	}
	n := models.Notification{ID: uuid.MustParse(id), NotificationType: models.NotificationAdminAnnouncement, Data: "null"}
	if got := WithNotificationNavigation(n).DestinationPath; got != "/notifications/"+id {
		t.Fatal(got)
	}
}
