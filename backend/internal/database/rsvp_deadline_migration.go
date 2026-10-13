package database

import (
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"time"
)

const RSVP48HourMigrationVersion = "notification-rsvp-48h-v1"

// ApplyRSVPDeadline48hWithin is part of the one-time R4 migration callback. The
// caller invokes RunNotificationMigration with RSVP48HourMigrationVersion and
// records reminder-stage skips in the SAME transaction and captured cutoff.
// Stored resolved starts and elapsed SQL hours retain Sydney DST correctness.
func ApplyRSVPDeadline48hWithin(tx *gorm.DB, cutoff time.Time) error {
	return tx.Model(&models.Session{}).
		Where("status = ? AND starts_at > ? AND rsvp_deadline > ?", models.SessionStatusOpen, cutoff, cutoff).
		Where("NOT EXISTS (SELECT 1 FROM settlements WHERE settlements.session_id = sessions.id)").
		UpdateColumn("rsvp_deadline", gorm.Expr("starts_at - INTERVAL '48 hours'")).Error
}
