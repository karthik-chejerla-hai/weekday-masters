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

	err := s.withSessionLock(input.SessionID, func(tx *gorm.DB, session models.Session) ([]uuid.UUID, error) {
		if session.Status != models.SessionStatusOpen {
			return nil, errors.New("session is not open for RSVPs")
		}

		now := utils.NowInSydney()
		isLate := now.After(session.RSVPDeadline)

		if !byAdmin && isLate {
			return nil, errors.New("RSVP deadline has passed")
		}

		existing := true
		result := tx.Where("session_id = ? AND user_id = ?", input.SessionID, input.UserID).First(&rsvp)
		if result.Error != nil {
			if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return nil, result.Error
			}
			existing = false
		}

		if existing && !byAdmin && isLate &&
			rsvp.Status == models.RSVPStatusIn && input.Status != models.RSVPStatusIn {
			return nil, errors.New("cannot change RSVP from IN after deadline")
		}

		wasConfirmed := existing && rsvp.Status == models.RSVPStatusIn

		// Apply the capacity rule. Admin-added players bypass it.
		status := input.Status
		if status == models.RSVPStatusIn && !byAdmin {
			confirmed, err := countConfirmed(tx, input.SessionID)
			if err != nil {
				return nil, err
			}
			if wasConfirmed {
				// The spot we already hold is not competition for ourselves.
				confirmed--
			}

			if confirmed >= session.MaxPlayers {
				status = models.RSVPStatusWaitlisted
			}
		}

		if existing {
			rsvp.Status = status
			if byAdmin {
				rsvp.AddedByAdmin = true
			}
			if err := tx.Save(&rsvp).Error; err != nil {
				return nil, err
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
				return nil, err
			}
		}

		// Giving up a confirmed spot lets the next waitlisted player in.
		if wasConfirmed && status != models.RSVPStatusIn {
			return promoteWithinTx(tx, session)
		}

		return nil, nil
	})

	if err != nil {
		return nil, err
	}

	if err := database.DB.Preload("User").First(&rsvp, "id = ?", rsvp.ID).Error; err != nil {
		return nil, err
	}
	s.fillWaitlistPosition(&rsvp)

	return &rsvp, nil
}

// DeleteRSVP removes an RSVP, promoting from the waitlist if a spot is freed.
func (s *RSVPService) DeleteRSVP(sessionID, userID uuid.UUID, byAdmin bool) error {
	return s.withSessionLock(sessionID, func(tx *gorm.DB, session models.Session) ([]uuid.UUID, error) {
		var rsvp models.RSVP
		if err := tx.Where("session_id = ? AND user_id = ?", sessionID, userID).First(&rsvp).Error; err != nil {
			return nil, errors.New("RSVP not found")
		}

		isLate := utils.NowInSydney().After(session.RSVPDeadline)
		if !byAdmin && isLate && rsvp.Status == models.RSVPStatusIn {
			return nil, errors.New("cannot remove IN RSVP after deadline")
		}

		wasConfirmed := rsvp.Status == models.RSVPStatusIn

		if err := tx.Delete(&rsvp).Error; err != nil {
			return nil, err
		}

		if wasConfirmed {
			return promoteWithinTx(tx, session)
		}

		return nil, nil
	})
}

// PromoteFromWaitlist fills any free spots from the waitlist. Call it after the
// session's capacity grows (for example when an admin adds a court).
func (s *RSVPService) PromoteFromWaitlist(sessionID uuid.UUID) error {
	return s.withSessionLock(sessionID, promoteWithinTx)
}

// withSessionLock runs fn with the session row locked FOR UPDATE, so concurrent
// RSVPs cannot both claim the last spot. Players fn promotes are notified only
// after the transaction commits.
func (s *RSVPService) withSessionLock(
	sessionID uuid.UUID,
	fn func(tx *gorm.DB, session models.Session) ([]uuid.UUID, error),
) error {
	var session models.Session
	var promoted []uuid.UUID

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&session, "id = ?", sessionID).Error; err != nil {
			return errors.New("session not found")
		}

		var err error
		promoted, err = fn(tx, session)
		return err
	})

	if err != nil {
		return err
	}

	s.notifyPromoted(promoted, session)
	return nil
}

// countConfirmed counts the players holding a confirmed spot in a session.
func countConfirmed(db *gorm.DB, sessionID uuid.UUID) (int, error) {
	var n int64
	if err := db.Model(&models.RSVP{}).
		Where("session_id = ? AND status = ?", sessionID, models.RSVPStatusIn).
		Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}

// promoteWithinTx moves the longest-waiting players into any free spots. The caller
// must already hold the session row lock.
func promoteWithinTx(tx *gorm.DB, session models.Session) ([]uuid.UUID, error) {
	confirmed, err := countConfirmed(tx, session.ID)
	if err != nil {
		return nil, err
	}

	spots := session.MaxPlayers - confirmed
	if spots <= 0 {
		return nil, nil
	}

	var waiting []models.RSVP
	if err := tx.Where("session_id = ? AND status = ?", session.ID, models.RSVPStatusWaitlisted).
		Order(waitlistOrder).
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

// waitlistOrder is the single definition of queue order: earliest RSVP first, with
// id breaking ties so promotion and displayed position always agree.
const waitlistOrder = "rsvp_timestamp ASC, id ASC"

// GetRSVPsForSession returns all RSVPs for a session, ordered by timestamp
func (s *RSVPService) GetRSVPsForSession(sessionID uuid.UUID) ([]models.RSVP, error) {
	var rsvps []models.RSVP
	if err := database.DB.Where("session_id = ?", sessionID).
		Preload("User").
		Order(waitlistOrder).
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

// fillWaitlistPosition sets the 1-based queue position on a waitlisted RSVP, reading
// it off the same ordered queue that promotion uses.
func (s *RSVPService) fillWaitlistPosition(rsvp *models.RSVP) {
	if rsvp.Status != models.RSVPStatusWaitlisted {
		rsvp.WaitlistPosition = 0
		return
	}

	var waiting []models.RSVP
	if err := database.DB.
		Where("session_id = ? AND status = ?", rsvp.SessionID, models.RSVPStatusWaitlisted).
		Order(waitlistOrder).
		Find(&waiting).Error; err != nil {
		return
	}

	AssignWaitlistPositions(waiting)
	for _, w := range waiting {
		if w.ID == rsvp.ID {
			rsvp.WaitlistPosition = w.WaitlistPosition
			return
		}
	}
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

	var rows []struct {
		Status models.RSVPStatus
		N      int
	}
	if err := database.DB.Model(&models.RSVP{}).
		Select("status, count(*) as n").
		Where("session_id = ?", sessionID).
		Group("status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	counts := make(map[models.RSVPStatus]int, len(rows))
	for _, r := range rows {
		counts[r.Status] = r.N
	}

	spotsLeft := session.MaxPlayers - counts[models.RSVPStatusIn]
	if spotsLeft < 0 {
		spotsLeft = 0
	}

	return &RSVPSummary{
		TotalIn:         counts[models.RSVPStatusIn],
		TotalOut:        counts[models.RSVPStatusOut],
		TotalMaybe:      counts[models.RSVPStatusMaybe],
		TotalWaitlisted: counts[models.RSVPStatusWaitlisted],
		MaxPlayers:      session.MaxPlayers,
		SpotsLeft:       spotsLeft,
	}, nil
}

// GetConfirmedPlayers returns players who have RSVP'd IN, ordered by timestamp
func (s *RSVPService) GetConfirmedPlayers(sessionID uuid.UUID) ([]models.RSVP, error) {
	var rsvps []models.RSVP
	if err := database.DB.Where("session_id = ? AND status = ?", sessionID, models.RSVPStatusIn).
		Preload("User").
		Order(waitlistOrder).
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
