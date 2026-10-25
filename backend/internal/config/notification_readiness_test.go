package config

import "testing"

func TestNotificationReadiness(t *testing.T) {
	for _, tc := range []struct {
		name, mode               string
		disabled, worker, reject bool
	}{
		{"enabled without worker", "release", false, false, true},
		{"stopped preparatory deployment", "release", true, false, false},
		{"worker provisioned", "release", false, true, false},
		{"local development", "debug", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := Config{GinMode: tc.mode, NotificationsDisabled: tc.disabled, NotificationWorkerEnabled: tc.worker}
			if (c.ValidateNotificationDeliveryReadiness() != nil) != tc.reject {
				t.Fatal("incorrect readiness decision")
			}
		})
	}
}
