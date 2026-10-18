package database

import (
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"time"
)

func MigrateRSVPReminderSchema(db *gorm.DB) error { return db.AutoMigrate(&models.RSVPReminderStage{}) }

// ApplyRSVPReminderCutoverWithin must follow deadline normalization inside the
// SAME versioned migration transaction. Already due stages are terminal skips;
// a redeployment cannot rewrite published/skipped stages or create a backlog.
func ApplyRSVPReminderCutoverWithin(tx *gorm.DB, cutoff time.Time) error {
	for _, stage := range []int{1, 2} {
		hours := 96
		if stage == 2 {
			hours = 60
		}
		expiry := "LEAST(starts_at - INTERVAL '60 hours', rsvp_deadline)"
		if stage == 2 {
			expiry = "rsvp_deadline"
		}
		due := "starts_at - (? * INTERVAL '1 hour')"
		statement := `INSERT INTO rsvp_reminder_stages (id,session_id,stage,state,starts_at,due_at,expires_at,skip_reason,created_at,updated_at)
   SELECT gen_random_uuid(),id,?,CASE WHEN ` + due + ` <= ? OR ` + expiry + ` <= ? THEN 'skipped' ELSE 'pending' END,
    starts_at,` + due + `,` + expiry + `,CASE WHEN ` + due + ` <= ? THEN 'rollout_past_due' WHEN ` + expiry + ` <= ? THEN 'stage_expired' ELSE '' END,?,?
   FROM sessions WHERE status = 'open' AND starts_at > ?
   ON CONFLICT (session_id,stage) DO NOTHING`
		if err := tx.Exec(statement, stage, hours, cutoff, cutoff, hours, hours, cutoff, cutoff, cutoff, cutoff, cutoff).Error; err != nil {
			return err
		}
	}
	return nil
}
