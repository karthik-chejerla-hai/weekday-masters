package services

import (
	"time"

	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Always lock club settings before sessions. Session creation, edits and
// recurring refresh use the same lock so a default change cannot miss them.
func lockClub(tx *gorm.DB) (*models.Club, error) {
	var club models.Club
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&club).Error; err != nil {
		return nil, err
	}
	return &club, nil
}

// UpdateClubSettings replaces future deadlines only when the default changes.
// A bulk SQL update preserves all session data and attendance history.
func UpdateClubSettings(updates map[string]any) (*models.Club, error) {
	var result *models.Club
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		club, err := lockClub(tx)
		if err != nil {
			return err
		}
		if hours, ok := updates["rsvp_deadline_hours"].(int); ok && hours != club.RSVPDeadlineHours {
			if hours < 0 || hours > 8760 {
				return gorm.ErrInvalidData
			}
			now := time.Now()
			if err := tx.Model(&models.Session{}).Where("starts_at > ?", now).
				Updates(map[string]any{"rsvp_deadline": gorm.Expr("starts_at - (? * INTERVAL '1 hour')", hours), "rsvp_deadline_custom": false, "updated_at": now}).Error; err != nil {
				return err
			}
			// Future occurrences of an older series must also use the new default.
			// This changes only metadata on past parents, never their stored deadline.
			if err := tx.Model(&models.Session{}).Where("is_recurring = ? AND rsvp_deadline_custom = ?", true, true).
				UpdateColumn("rsvp_deadline_custom", false).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(club).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.First(club, "id = ?", club.ID).Error; err != nil {
			return err
		}
		result = club
		return nil
	})
	return result, err
}
