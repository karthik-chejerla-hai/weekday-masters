package services

import (
	"context"
	"errors"
	"log"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WaitlistNotifier is the slice of NotificationService that RSVPService needs in
// order to tell players they have been promoted off the waitlist.
type WaitlistNotifier interface {
	SendNotification(ctx context.Context, userID uuid.UUID, notifType models.NotificationType, title, body string, data map[string]string) error
}

type RSVPService struct {
	notifier WaitlistNotifier
}

func NewRSVPService(notifier WaitlistNotifier) *RSVPService {
	return &RSVPService{notifier: notifier}
}

type RSVPInput struct {
	SessionID uuid.UUID
	UserID    uuid.UUID
	Status    models.RSVPStatus
}

// CreateOrUpdateRSVP creates or updates an RSVP.
//
// Capacity is enforced here: once a session has MaxPlayers confirmed players, further
// "in" requests are accepted as "waitlisted" instead. Admins bypass the cap. Freeing a
// confirmed spot promotes the longest-waiting player automatically.
func (s *RSVPService) CreateOrUpdateRSVP(input RSVPInput, byAdmin bool) (*models.RSVP, error) {
	var rsvp models.RSVP
	var promoted []uuid.UUID
	var session models.Session

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Lock the session row so concurrent RSVPs cannot both claim the last spot.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&session, "id = ?", input.SessionID).Error; err != nil {
			return errors.New("session not found")
		}

		if session.Status != models.SessionStatusOpen {
			return errors.New("session is not open for RSVPs")
		}

		now := utils.NowInSydney()
		isLate := now.After(session.RSVPDeadline)

		if !byAdmin && isLate {
			return errors.New("RSVP deadline has passed")
		}

		existing := true
		result := tx.Where("session_id = ? AND user_id = ?", input.SessionID, input.UserID).First(&rsvp)
		if result.Error != nil {
			if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return result.Error
			}
			existing = false
		}

		if existing && !byAdmin && isLate &&
			rsvp.Status == models.RSVPStatusIn && input.Status != models.RSVPStatusIn {
			return errors.New("cannot change RSVP from IN after deadline")
		}

		wasConfirmed := existing && rsvp.Status == models.RSVPStatusIn

		// Apply the capacity rule. Admin-added players bypass it.
		status := input.Status
		if status == models.RSVPStatusIn && !byAdmin {
			var confirmed int64
			if err := tx.Model(&models.RSVP{}).
				Where("session_id = ? AND status = ? AND user_id <> ?",
					input.SessionID, models.RSVPStatusIn, input.UserID).
				Count(&confirmed).Error; err != nil {
				return err
			}

			if int(confirmed) >= session.MaxPlayers {
				status = models.RSVPStatusWaitlisted
			}
		}

		if existing {
			rsvp.Status = status
			if byAdmin {
				rsvp.AddedByAdmin = true
			}
			if err := tx.Save(&rsvp).Error; err != nil {
				return err
			}
		} else {
			rsvp = models.RSVP{
				SessionID:     input.SessionID,
				UserID:        input.UserID,
				Status:        status,
				RSVPTimestamp: now,
				IsLateRSVP:    isLate,
				AddedByAdmin:  byAdmin,
			}
			if err := tx.Create(&rsvp).Error; err != nil {
				return err
			}
		}

		// Giving up a confirmed spot lets the next waitlisted player in.
		if wasConfirmed && status != models.RSVPStatusIn {
			ids, err := promoteWithinTx(tx, session)
			if err != nil {
				return err
			}
			promoted = ids
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	s.notifyPromoted(promoted, session)

	if err := database.DB.Preload("User").First(&rsvp, "id = ?", rsvp.ID).Error; err != nil {
		return nil, err
	}
	s.fillWaitlistPosition(&rsvp)

	return &rsvp, nil
}

// DeleteRSVP removes an RSVP, promoting from the waitlist if a spot is freed.
func (s *RSVPService) DeleteRSVP(sessionID, userID uuid.UUID, byAdmin bool) error {
	var promoted []uuid.UUID
	var session models.Session

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&session, "id = ?", sessionID).Error; err != nil {
			return errors.New("session not found")
		}

		var rsvp models.RSVP
		if err := tx.Where("session_id = ? AND user_id = ?", sessionID, userID).First(&rsvp).Error; err != nil {
			return errors.New("RSVP not found")
		}

		isLate := utils.NowInSydney().After(session.RSVPDeadline)
		if !byAdmin && isLate && rsvp.Status == models.RSVPStatusIn {
			return errors.New("cannot remove IN RSVP after deadline")
		}

		wasConfirmed := rsvp.Status == models.RSVPStatusIn

		if err := tx.Delete(&rsvp).Error; err != nil {
			return err
		}

		if wasConfirmed {
			ids, err := promoteWithinTx(tx, session)
			if err != nil {
				return err
			}
			promoted = ids
		}

		return nil
	})

	if err != nil {
		return err
	}

	s.notifyPromoted(promoted, session)
	return nil
}

// PromoteFromWaitlist fills any free spots from the waitlist. Call it after the
// session's capacity grows (for example when an admin adds a court).
func (s *RSVPService) PromoteFromWaitlist(sessionID uuid.UUID) error {
	var promoted []uuid.UUID
	var session models.Session

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&session, "id = ?", sessionID).Error; err != nil {
			return err
		}

		ids, err := promoteWithinTx(tx, session)
		if err != nil {
			return err
		}
		promoted = ids
		return nil
	})

	if err != nil {
		return err
	}

	s.notifyPromoted(promoted, session)
	return nil
}

// promoteWithinTx moves the longest-waiting players into any free spots. The caller
// must already hold the session row lock.
func promoteWithinTx(tx *gorm.DB, session models.Session) ([]uuid.UUID, error) {
	var confirmed int64
	if err := tx.Model(&models.RSVP{}).
		Where("session_id = ? AND status = ?", session.ID, models.RSVPStatusIn).
		Count(&confirmed).Error; err != nil {
		return nil, err
	}

	spots := session.MaxPlayers - int(confirmed)
	if spots <= 0 {
		return nil, nil
	}

	var waiting []models.RSVP
	if err := tx.Where("session_id = ? AND status = ?", session.ID, models.RSVPStatusWaitlisted).
		Order("rsvp_timestamp ASC, id ASC").
		Limit(spots).
		Find(&waiting).Error; err != nil {
		return nil, err
	}

	if len(waiting) == 0 {
		return nil, nil
	}

	ids := make([]uuid.UUID, 0, len(waiting))
	rsvpIDs := make([]uuid.UUID, 0, len(waiting))
	for _, r := range waiting {
		ids = append(ids, r.UserID)
		rsvpIDs = append(rsvpIDs, r.ID)
	}

	if err := tx.Model(&models.RSVP{}).
		Where("id IN ?", rsvpIDs).
		Update("status", models.RSVPStatusIn).Error; err != nil {
		return nil, err
	}

	return ids, nil
}

// notifyPromoted tells players they have moved off the waitlist. Failures are logged
// but never surfaced — the promotion itself is already committed.
func (s *RSVPService) notifyPromoted(userIDs []uuid.UUID, session models.Session) {
	if len(userIDs) == 0 || s.notifier == nil {
		return
	}

	ctx := context.Background()
	dateStr := utils.FormatDateForDisplay(session.SessionDate)
	title := "You're in!"
	body := "A spot opened up for " + session.Title + " on " + dateStr + ". You're off the waitlist and confirmed to play."
	data := map[string]string{
		"type":       string(models.NotificationWaitlistUpdate),
		"session_id": session.ID.String(),
	}

	for _, userID := range userIDs {
		if err := s.notifier.SendNotification(ctx, userID, models.NotificationWaitlistUpdate, title, body, data); err != nil {
			log.Printf("Error notifying promoted user %s: %v", userID, err)
		}
	}
}

// GetRSVPsForSession returns all RSVPs for a session, ordered by timestamp
func (s *RSVPService) GetRSVPsForSession(sessionID uuid.UUID) ([]models.RSVP, error) {
	var rsvps []models.RSVP
	if err := database.DB.Where("session_id = ?", sessionID).
		Preload("User").
		Order("rsvp_timestamp ASC").
		Find(&rsvps).Error; err != nil {
		return nil, err
	}

	AssignWaitlistPositions(rsvps)

	return rsvps, nil
}

// GetUserRSVPForSession returns a user's RSVP for a session
func (s *RSVPService) GetUserRSVPForSession(sessionID, userID uuid.UUID) (*models.RSVP, error) {
	var rsvp models.RSVP
	if err := database.DB.Where("session_id = ? AND user_id = ?", sessionID, userID).
		First(&rsvp).Error; err != nil {
		return nil, err
	}
	s.fillWaitlistPosition(&rsvp)
	return &rsvp, nil
}

// fillWaitlistPosition sets the 1-based queue position on a waitlisted RSVP.
func (s *RSVPService) fillWaitlistPosition(rsvp *models.RSVP) {
	if rsvp.Status != models.RSVPStatusWaitlisted {
		rsvp.WaitlistPosition = 0
		return
	}

	var ahead int64
	database.DB.Model(&models.RSVP{}).
		Where("session_id = ? AND status = ? AND (rsvp_timestamp < ? OR (rsvp_timestamp = ? AND id < ?))",
			rsvp.SessionID, models.RSVPStatusWaitlisted, rsvp.RSVPTimestamp, rsvp.RSVPTimestamp, rsvp.ID).
		Count(&ahead)

	rsvp.WaitlistPosition = int(ahead) + 1
}

// RSVPSummary contains summary statistics for a session's RSVPs
type RSVPSummary struct {
	TotalIn         int `json:"total_in"`
	TotalOut        int `json:"total_out"`
	TotalMaybe      int `json:"total_maybe"`
	TotalWaitlisted int `json:"total_waitlisted"`
	MaxPlayers      int `json:"max_players"`
	SpotsLeft       int `json:"spots_left"`
}

// GetRSVPSummary returns summary statistics for a session
func (s *RSVPService) GetRSVPSummary(sessionID uuid.UUID) (*RSVPSummary, error) {
	var session models.Session
	if err := database.DB.First(&session, "id = ?", sessionID).Error; err != nil {
		return nil, err
	}

	counts := map[models.RSVPStatus]int64{}
	for _, status := range []models.RSVPStatus{
		models.RSVPStatusIn,
		models.RSVPStatusOut,
		models.RSVPStatusMaybe,
		models.RSVPStatusWaitlisted,
	} {
		var count int64
		database.DB.Model(&models.RSVP{}).
			Where("session_id = ? AND status = ?", sessionID, status).
			Count(&count)
		counts[status] = count
	}

	spotsLeft := session.MaxPlayers - int(counts[models.RSVPStatusIn])
	if spotsLeft < 0 {
		spotsLeft = 0
	}

	return &RSVPSummary{
		TotalIn:         int(counts[models.RSVPStatusIn]),
		TotalOut:        int(counts[models.RSVPStatusOut]),
		TotalMaybe:      int(counts[models.RSVPStatusMaybe]),
		TotalWaitlisted: int(counts[models.RSVPStatusWaitlisted]),
		MaxPlayers:      session.MaxPlayers,
		SpotsLeft:       spotsLeft,
	}, nil
}

// GetConfirmedPlayers returns players who have RSVP'd IN, ordered by timestamp
func (s *RSVPService) GetConfirmedPlayers(sessionID uuid.UUID) ([]models.RSVP, error) {
	var rsvps []models.RSVP
	if err := database.DB.Where("session_id = ? AND status = ?", sessionID, models.RSVPStatusIn).
		Preload("User").
		Order("rsvp_timestamp ASC").
		Find(&rsvps).Error; err != nil {
		return nil, err
	}
	return rsvps, nil
}

// AssignWaitlistPositions numbers the waitlisted entries in a slice that is already
// ordered by rsvp_timestamp ascending.
func AssignWaitlistPositions(rsvps []models.RSVP) {
	position := 0
	for i := range rsvps {
		if rsvps[i].Status == models.RSVPStatusWaitlisted {
			position++
			rsvps[i].WaitlistPosition = position
		}
	}
}
