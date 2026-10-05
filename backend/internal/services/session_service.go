package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
}

func (s *SessionService) WithNotifier(notifier *NotificationService) *SessionService {
	s.notifier = notifier
	return s
}

func NewSessionService() *SessionService {
	return &SessionService{}
}

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
	RSVPDeadline       *time.Time // Optional custom deadline; defaults to 3 days before session
}

// CreateSession creates a new session
func (s *SessionService) CreateSession(input CreateSessionInput) (*models.Session, error) {
	if input.Courts < 1 || input.Courts > 3 {
		return nil, errors.New("courts must be between 1 and 3")
	}

	// Determine RSVP deadline. Only an explicitly supplied deadline is validated:
	// the 3-day default lands in the past for short-notice sessions, and admins
	// must still be able to create those.
	rsvpDeadline := utils.CalculateRSVPDeadline(input.SessionDate)
	if input.RSVPDeadline != nil {
		if input.RSVPDeadline.Before(utils.NowInSydney()) {
			return nil, errors.New("RSVP deadline cannot be in the past")
		}
		rsvpDeadline = *input.RSVPDeadline
	}

	session := models.Session{
		Title:              input.Title,
		Description:        input.Description,
		SessionDate:        input.SessionDate,
		StartTime:          input.StartTime,
		EndTime:            input.EndTime,
		Courts:             input.Courts,
		MaxPlayers:         models.MaxPlayersForCourts(input.Courts),
		RSVPDeadline:       rsvpDeadline,
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

	// Derive the relative deadline offset from the parent session.
	// This preserves any custom deadline the admin set on the parent.
	parentDateSyd := parent.SessionDate.In(utils.SydneyLocation)
	parentDeadlineSyd := parent.RSVPDeadline.In(utils.SydneyLocation)

	// Count whole days in UTC: subtracting two Sydney midnights across a DST
	// boundary yields 71 or 73 hours and would truncate to the wrong offset.
	sessionDay := time.Date(parentDateSyd.Year(), parentDateSyd.Month(), parentDateSyd.Day(), 0, 0, 0, 0, time.UTC)
	deadlineDay := time.Date(parentDeadlineSyd.Year(), parentDeadlineSyd.Month(), parentDeadlineSyd.Day(), 0, 0, 0, 0, time.UTC)
	daysBefore := int(sessionDay.Sub(deadlineDay).Hours() / 24)
	deadlineHour := parentDeadlineSyd.Hour()
	deadlineMin := parentDeadlineSyd.Minute()
	deadlineSec := parentDeadlineSyd.Second()

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

			// Compute child deadline using the same relative offset as the parent
			childDateSyd := nextDate.In(utils.SydneyLocation)
			childDeadline := time.Date(
				childDateSyd.Year(), childDateSyd.Month(), childDateSyd.Day()-daysBefore,
				deadlineHour, deadlineMin, deadlineSec, 0,
				utils.SydneyLocation,
			)

			child := models.Session{
				Title:             childTitle,
				Description:       parent.Description,
				SessionDate:       nextDate,
				StartTime:         parent.StartTime,
				EndTime:           parent.EndTime,
				Courts:            parent.Courts,
				MaxPlayers:        parent.MaxPlayers,
				RSVPDeadline:      childDeadline,
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
	RSVPDeadline *time.Time // Optional; if set, overrides auto-calculated deadline
}

// UpdateSession updates a session
func (s *SessionService) UpdateSession(id uuid.UUID, input UpdateSessionInput) (*models.Session, error) {
	var session models.Session
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, "id = ?", id).Error; err != nil {
			return err
		}
		if session.Status == models.SessionStatusCancelled {
			return errors.New("cancelled sessions cannot be edited")
		}
		if input.Status != nil && *input.Status != models.SessionStatusOpen && *input.Status != models.SessionStatusClosed {
			return errors.New("use the cancel action to cancel a session")
		}

		if input.Title != nil {
			session.Title = *input.Title
		}
		if input.Description != nil {
			session.Description = *input.Description
		}
		if input.SessionDate != nil {
			session.SessionDate = *input.SessionDate
			// Only auto-recalculate deadline if no explicit deadline is provided
			if input.RSVPDeadline == nil {
				session.RSVPDeadline = utils.CalculateRSVPDeadline(*input.SessionDate)
			}
		}
		if input.RSVPDeadline != nil {
			if input.RSVPDeadline.Before(utils.NowInSydney()) {
				return errors.New("RSVP deadline cannot be in the past")
			}
			session.RSVPDeadline = *input.RSVPDeadline
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
		var rsvpCount int64
		if err := tx.Model(&models.RSVP{}).Where("session_id = ?", id).Count(&rsvpCount).Error; err != nil {
			return err
		}
		if rsvpCount > 0 {
			return errors.New("this session has RSVPs; use the cancel action to notify members")
		}
		return tx.Delete(&session).Error
	})
}

const MaxCancellationReasonLength = 1000

// CancelSession retains the schedule and attendance history. The session lock
// serializes cancellation with RSVP, editing and settlement. Notifications are
// persisted in the same transaction, then delivered only by the winning request.
func (s *SessionService) CancelSession(id uuid.UUID, reason string, actorID uuid.UUID) (*models.Session, error) {
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > MaxCancellationReasonLength {
		return nil, errors.New("cancellation reason must be at most 1000 characters")
	}
	var session models.Session
	var notifications []models.Notification
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
		if err := tx.Create(&announcement).Error; err != nil {
			return err
		}
		var members []models.User
		if err := tx.Select("id").Where("membership_status = ?", models.MembershipApproved).Find(&members).Error; err != nil {
			return err
		}
		data, _ := json.Marshal(map[string]string{
			"type":            string(models.NotificationAdminAnnouncement),
			"announcement_id": announcement.ID.String(),
			"session_id":      id.String(),
		})
		for _, member := range members {
			notifications = append(notifications, models.Notification{
				UserID: member.ID, NotificationType: models.NotificationAdminAnnouncement,
				Title: announcement.Title, Body: announcement.Body, Data: string(data),
			})
		}
		if len(notifications) > 0 {
			return tx.Create(&notifications).Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if s.notifier != nil && len(notifications) > 0 {
		// A disconnected client must not cancel delivery after the write commits.
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		for i := range notifications {
			if err := s.notifier.deliverNotification(ctx, &notifications[i]); err != nil {
				log.Printf("Failed to deliver cancellation notification %s: %v", notifications[i].ID, err)
			}
		}
	}
	return &session, nil
}
