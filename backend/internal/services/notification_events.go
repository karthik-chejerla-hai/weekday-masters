package services

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/models"
)

// NotificationDestination derives trusted local routes from typed source IDs.
// No arbitrary URL in a stored/provider payload can become a navigation target.
func NotificationDestination(data map[string]string) string {
	if id, err := uuid.Parse(data["session_id"]); err == nil {
		if data["type"] == "session_charge" || data["type"] == "session_charge_reversed" {
			return "/sessions/" + id.String() + "/settlement"
		}
		return "/sessions/" + id.String()
	}
	switch data["type"] {
	case "push_test":
		return "/dashboard"
	case "balance_low", "balance_negative":
		return "/money"
	}
	if id, err := uuid.Parse(data["notification_id"]); err == nil {
		return "/notifications/" + id.String()
	}
	return "/notifications"
}

// NotificationNavigation is additive: original data and notification_type remain unchanged.
type NotificationNavigation struct {
	models.Notification
	DestinationPath string `json:"destination_path"`
}

func WithNotificationNavigation(n models.Notification) NotificationNavigation {
	data := map[string]string{}
	_ = json.Unmarshal([]byte(n.Data), &data)
	if data == nil {
		data = map[string]string{}
	}
	data["type"] = string(n.NotificationType)
	data["notification_id"] = n.ID.String()
	return NotificationNavigation{n, NotificationDestination(data)}
}
