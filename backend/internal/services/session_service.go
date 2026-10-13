package services

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SessionService struct {
	notifier *NotificationService
	now      func() time.Time
}

func (s *SessionService) WithNotifier(notifier *NotificationService) *SessionService {
	s.notifier = notifier
	return s
}

func NewSessionService() *SessionService {
	return &SessionService{}
}

func (s *SessionService) currentTime() time.Time {
	if s.now != nil {
		return s.now()
	}
	return utils.NowInSydney()
}

var ErrFixedRSVPDeadline = errors.New("RSVP deadline is fixed at 48 hours before session start")

type CreateSessionInput struct {
	Title              string
	Description        string
	SessionDate        time.Time
	StartTime          string
	EndTime            string
	Courts             int
	IsRecurring        bool
	RecurringDayOfWeek *int
	Occurrences        *int
	CreatedBy          uuid.UUID
	RSVPDeadline       *time.Time // Retained only to reject obsolete custom-deadline callers
}

// CreateSession creates a new session
func (s *SessionService) CreateSession(input CreateSessionInput) (*models.Session, error) {
	if input.Courts < 1 || input.Courts > 3 {
		return nil, errors.New("courts must be between 1 and 3")
	}

	if input.RSVPDeadline != nil {
		return nil, ErrFixedRSVPDeadline
	}
	startsAt, endsAt, err := utils.ResolveSessionTimes(input.SessionDate, input.StartTime, input.EndTime)
	if err != nil {
		return nil, err
	}
	// Short-notice sessions may already have a closed window; admins can still
	// create them and use the existing admin RSVP exception.
	rsvpDeadline := utils.CalculateRSVPDeadline(startsAt)

	session := models.Session{
		Title:              input.Title,
		Description:        input.Description,
		SessionDate:        input.SessionDate,
		StartTime:          input.StartTime,
		EndTime:            input.EndTime,
		Courts:             input.Courts,
		MaxPlayers:         models.MaxPlayersForCourts(input.Courts),
		RSVPDeadline:       rsvpDeadline,
		StartsAt:           &startsAt,
		EndsAt:             &endsAt,
		IsRecurring:        input.IsRecurring,
		RecurringDayOfWeek: input.RecurringDayOfWeek,
		Status:             models.SessionStatusOpen,
		CreatedBy:          input.CreatedBy,
	}

	if err := database.DB.Create(&session).Error; err != nil {
		return nil, err
	}

	// If recurring, generate sessions for the specified number of occurrences
	if input.IsRecurring && input.RecurringDayOfWeek != nil {
		occurrences := 4 // default
		if input.Occurrences != nil && *input.Occurrences > 0 {
			occurrences = *input.Occurrences
		}
		s.generateRecurringSessions(&session, occurrences)
	}

	return &session, nil
}

// generateRecurringSessions creates recurring session instances
func (s *SessionService) generateRecurringSessions(parent *models.Session, occurrences int) error {
	if parent.RecurringDayOfWeek == nil {
		return nil
	}

	// Start from the next week after the parent session
	nextDate := parent.SessionDate.AddDate(0, 0, 7)

	// Generate sessions for the specified number of occurrences (minus 1 since parent counts as first)
	for i := 0; i < occurrences-1; i++ {
		// Check if session already exists
		var count int64
		database.DB.Model(&models.Session{}).
			Where("session_date = ? AND recurring_parent_id = ?", nextDate, parent.ID).
			Count(&count)

		if count == 0 {
			// Generate title for this occurrence in format "Day - DD MMM YYYY"
			childTitle := nextDate.Format("Monday - 02 Jan 2006")

			startsAt, endsAt, err := utils.ResolveSessionTimes(nextDate, parent.StartTime, parent.EndTime)
			if err != nil {
				return err
			}
			childDeadline := utils.CalculateRSVPDeadline(startsAt)

			child := models.Session{
				Title:             childTitle,
				Description:       parent.Description,
				SessionDate:       nextDate,
				StartTime:         parent.StartTime,
				EndTime:           parent.EndTime,
				Courts:            parent.Courts,
				MaxPlayers:        parent.MaxPlayers,
				RSVPDeadline:      childDeadline,
				StartsAt:          &startsAt,
				EndsAt:            &endsAt,
				IsRecurring:       false,
				RecurringParentID: &parent.ID,
				Status:            models.SessionStatusOpen,
				CreatedBy:         parent.CreatedBy,
			}
			database.DB.Create(&child)
		}

		nextDate = nextDate.AddDate(0, 0, 7)
	}

	return nil
}

// RefreshRecurringSessions generates any missing recurring session instances
// This is called for maintenance/refresh - uses default of 4 weeks ahead
func (s *SessionService) RefreshRecurringSessions() error {
	var parentSessions []models.Session
	if err := database.DB.Where("is_recurring = ? AND status = ?", true, models.SessionStatusOpen).
		Find(&parentSessions).Error; err != nil {
		return err
	}

	for _, parent := range parentSessions {
		s.generateRecurringSessions(&parent, 4) // Default to 4 weeks for refresh
	}

	return nil
}

// GetSessionByID retrieves a session by ID with RSVPs and user details
func (s *SessionService) GetSessionByID(id uuid.UUID) (*models.Session, error) {
	var session models.Session
	if err := database.DB.Preload("RSVPs", func(db *gorm.DB) *gorm.DB {
		return db.Order("rsvp_timestamp ASC")
	}).Preload("RSVPs.User").Preload("Creator").
		First(&session, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// ListUpcomingSessions returns upcoming sessions
func (s *SessionService) ListUpcomingSessions() ([]models.Session, error) {
	var sessions []models.Session
	now := utils.NowInSydney()
	today := utils.StartOfDay(now)

	// Upcoming means "has not finished", not "is dated today or later" — a
	// session that ended three hours ago is in the past, and the History tab
	// depends on that boundary being right.
	if err := database.DB.Where("(ends_at IS NULL OR ends_at >= ?) AND session_date >= ? AND status != ?", time.Now(), today, models.SessionStatusCancelled).
		Preload("RSVPs", func(db *gorm.DB) *gorm.DB {
			return db.Order("rsvp_timestamp ASC")
		}).
		Preload("RSVPs.User").
		Order("session_date ASC, start_time ASC").
		Find(&sessions).Error; err != nil {
		return nil, err
	}

	return sessions, nil
}

// ListCancelledUpcomingSessions returns cancelled sessions that haven't passed yet
func (s *SessionService) ListCancelledUpcomingSessions() ([]models.Session, error) {
	var sessions []models.Session
	now := utils.NowInSydney()
	today := utils.StartOfDay(now)

	if err := database.DB.Where("(ends_at IS NULL OR ends_at >= ?) AND session_date >= ? AND status = ?", time.Now(), today, models.SessionStatusCancelled).
		Preload("RSVPs").
		Order("session_date ASC, start_time ASC").
		Find(&sessions).Error; err != nil {
		return nil, err
	}

	return sessions, nil
}

type UpdateSessionInput struct {
	Title        *string
	Description  *string
	SessionDate  *time.Time
	StartTime    *string
	EndTime      *string
	Courts       *int
	Status       *models.SessionStatus
	RSVPDeadline *time.Time // Retained only to reject obsolete custom-deadline callers
}

// UpdateSession updates a session
func (s *SessionService) UpdateSession(id uuid.UUID, input UpdateSessionInput) (*models.Session, error) {
	if input.RSVPDeadline != nil {
		return nil, ErrFixedRSVPDeadline
	}
	var session models.Session
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, "id = ?", id).Error; err != nil {
			return err
		}
		if session.Status == models.SessionStatusCancelled {
			return errors.New("cancelled sessions cannot be edited")
		}
		if session.Status == models.SessionStatusClosed && input.Status != nil && *input.Status == models.SessionStatusOpen {
			return errors.New("closed RSVP windows cannot be reopened")
		}
		if input.Status != nil && *input.Status != models.SessionStatusOpen && *input.Status != models.SessionStatusClosed {
			return errors.New("use the cancel action to cancel a session")
		}

		now := s.currentTime()
		oldStart := session.StartsAt
		windowWasOpen := session.Status == models.SessionStatusOpen && session.RSVPDeadline.After(now) && oldStart != nil && oldStart.After(now)
		var settlements int64
		if windowWasOpen {
			if err := tx.Model(&models.Settlement{}).Where("session_id = ?", session.ID).Count(&settlements).Error; err != nil {
				return err
			}
		}

		if input.Title != nil {
			session.Title = *input.Title
		}
		if input.Description != nil {
			session.Description = *input.Description
		}
		if input.SessionDate != nil {
			session.SessionDate = *input.SessionDate
		}
		if input.StartTime != nil {
			session.StartTime = *input.StartTime
		}
		if input.EndTime != nil {
			session.EndTime = *input.EndTime
		}
		if input.Courts != nil {
			if *input.Courts < 1 || *input.Courts > 3 {
				return errors.New("courts must be between 1 and 3")
			}
			session.Courts = *input.Courts
			session.MaxPlayers = models.MaxPlayersForCourts(*input.Courts)
		}
		if input.Status != nil {
			session.Status = *input.Status
		}

		startsAt, endsAt, err := utils.ResolveSessionTimes(session.SessionDate, session.StartTime, session.EndTime)
		if err != nil {
			return err
		}
		session.StartsAt, session.EndsAt = &startsAt, &endsAt
		if windowWasOpen && settlements == 0 && !startsAt.Equal(*oldStart) {
			session.RSVPDeadline = utils.CalculateRSVPDeadline(startsAt)
		}
		// Already closed, started/historical or settled windows stay closed
		// even when the date moves forward or status is explicitly reopened.
		session.UpdatedAt = time.Now()

		return tx.Save(&session).Error
	})
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// DeleteSession only removes unused sessions. Cancellation must go through the
// explicit action so members are told, even when a session has no RSVPs.
func (s *SessionService) DeleteSession(id uuid.UUID) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var session models.Session
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, "id = ?", id).Error; err != nil {
			return err
		}
		if session.Status == models.SessionStatusCancelled {
			return errors.New("cancelled sessions must be retained")
		}
		var games int64
		if err := tx.Model(&models.GameResult{}).Where("session_id = ?", id).Count(&games).Error; err != nil {
			return err
		}
		if games > 0 {
			return gameConflict("This session has recorded games and cannot be deleted.")
		}
		var rsvpCount int64
		if err := tx.Model(&models.RSVP{}).Where("session_id = ?", id).Count(&rsvpCount).Error; err != nil {
			return err
		}
		if rsvpCount > 0 {
			return errors.New("this session has RSVPs; use the cancel action to notify members")
		}
		// A reminder scan may already have materialized stages and queued events
		// for this RSVP-free session. Keep member history, but prevent undelivered
		// reminder intents from firing after the session is removed.
		var reminderNotificationIDs []uuid.UUID
		if err := tx.Model(&models.Notification{}).
			Where("notification_type = ? AND data::jsonb ->> 'session_id' = ?", models.NotificationRSVPReminder, id.String()).
			Pluck("id", &reminderNotificationIDs).Error; err != nil {
			return err
		}
		if len(reminderNotificationIDs) > 0 {
			var outboxIDs []uuid.UUID
			if err := tx.Model(&models.NotificationOutbox{}).Where("notification_id IN ?", reminderNotificationIDs).Pluck("id", &outboxIDs).Error; err != nil {
				return err
			}
			if len(outboxIDs) > 0 {
				// Match the dispatcher lock order (outbox, then attempts) and
				// stabilize the worker lease before changing any per-device state.
				var outboxes []models.NotificationOutbox
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", outboxIDs).Order("id ASC").Find(&outboxes).Error; err != nil {
					return err
				}
				for _, outbox := range outboxes {
					if err := tx.Model(&models.NotificationDeliveryAttempt{}).
						Where("outbox_id = ? AND state IN ?", outbox.ID, []models.DeliveryState{models.DeliveryPending, models.DeliveryTransient}).
						Updates(map[string]any{"state": models.DeliverySuppressed, "last_error_code": "session_deleted"}).Error; err != nil {
						return err
					}
					if outbox.State == models.DeliveryPending || outbox.State == models.DeliveryTransient {
						var attempts []models.NotificationDeliveryAttempt
						if err := tx.Where("outbox_id = ?", outbox.ID).Find(&attempts).Error; err != nil {
							return err
						}
						accepted, uncertain, failed, suppressed, expired := 0, 0, 0, 0, 0
						for _, attempt := range attempts {
							switch attempt.State {
							case models.DeliveryAccepted:
								accepted++
							case models.DeliveryUncertain:
								uncertain++
							case models.DeliveryFailed:
								failed++
							case models.DeliverySuppressed:
								suppressed++
							case models.DeliveryExpired:
								expired++
							}
						}
						state := models.DeliverySuppressed
						switch {
						case accepted > 0 && uncertain+failed+suppressed+expired > 0:
							state = models.DeliveryPartial
						case accepted > 0:
							state = models.DeliveryCompleted
						case uncertain > 0:
							state = models.DeliveryUncertain
						case failed > 0:
							state = models.DeliveryFailed
						case expired > 0:
							state = models.DeliveryExpired
						}
						if err := tx.Model(&models.NotificationOutbox{}).Where("id = ?", outbox.ID).
							Updates(map[string]any{"state": state, "last_error_code": "session_deleted"}).Error; err != nil {
							return err
						}
					} else if outbox.State == models.DeliveryClaimed {
						// A live worker may be inside provider I/O with no DB lock.
						// Keep its lease whenever a send crossed the durable boundary;
						// it must record acceptance/uncertainty or lease recovery must.
						var inFlightOrTerminal int64
						if err := tx.Model(&models.NotificationDeliveryAttempt{}).
							Where("outbox_id = ? AND state IN ?", outbox.ID, []models.DeliveryState{models.DeliverySending, models.DeliveryAccepted, models.DeliveryUncertain}).
							Count(&inFlightOrTerminal).Error; err != nil {
							return err
						}
						if inFlightOrTerminal == 0 {
							if err := tx.Model(&models.NotificationOutbox{}).Where("id = ? AND state = ? AND lease_owner = ? AND lease_generation = ?", outbox.ID, models.DeliveryClaimed, outbox.LeaseOwner, outbox.LeaseGeneration).
								Updates(map[string]any{"state": models.DeliverySuppressed, "last_error_code": "session_deleted"}).Error; err != nil {
								return err
							}
						}
					}
				}
			}
		}
		if err := tx.Where("session_id = ?", id).Delete(&models.RSVPReminderStage{}).Error; err != nil {
			return err
		}
		return tx.Delete(&session).Error
	})
}

const MaxCancellationReasonLength = 1000

// CancelSession retains the schedule and attendance history. The session lock
// serializes cancellation with RSVP, editing and settlement. Notifications are
// persisted with durable delivery intent in the same transaction.
func (s *SessionService) CancelSession(id uuid.UUID, reason string, actorID uuid.UUID) (*models.Session, error) {
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > MaxCancellationReasonLength {
		return nil, errors.New("cancellation reason must be at most 1000 characters")
	}
	var session models.Session
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, "id = ?", id).Error; err != nil {
			return err
		}
		if session.Status == models.SessionStatusCancelled {
			return nil
		}
		now := time.Now()
		if session.StartsAt == nil || session.EndsAt == nil || !session.EndsAt.After(now) {
			return errors.New("finished sessions cannot be cancelled")
		}
		var settlements int64
		if err := tx.Model(&models.Settlement{}).Where("session_id = ?", id).Count(&settlements).Error; err != nil {
			return err
		}
		if settlements > 0 {
			return errors.New("settled sessions cannot be cancelled")
		}

		var next models.Session
		err := tx.Where("id <> ? AND status <> ? AND starts_at > ? AND starts_at > ?", id, models.SessionStatusCancelled, session.StartsAt, now).
			Order("starts_at ASC, id ASC").First(&next).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		nextText := "No next session is scheduled."
		if err == nil {
			nextText = "Next scheduled session: " + next.StartsAt.In(utils.SydneyLocation).Format("Monday, 2 January 2006 at 15:04 MST") + "."
		}
		reasonText := reason
		if reasonText == "" {
			reasonText = "No reason provided."
		}
		announcement := models.Announcement{
			Title:     "Session cancelled",
			Body:      fmt.Sprintf("Cancelled session: %s.\nReason: %s\n%s", session.StartsAt.In(utils.SydneyLocation).Format("Monday, 2 January 2006 at 15:04 MST"), reasonText, nextText),
			CreatedBy: actorID,
		}
		session.Status = models.SessionStatusCancelled
		session.CancellationReason = reason
		if err := tx.Save(&session).Error; err != nil {
			return err
		}
		_, err = NewAnnouncementService(s.notifier).publishWithin(tx, actorID, "session-cancelled:"+id.String(), nil, announcement.Title, announcement.Body, &id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &session, nil
}
