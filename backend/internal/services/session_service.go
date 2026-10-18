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
	RSVPDeadline       *time.Time // Optional custom deadline; otherwise uses the club default
}

// CreateSession creates a new session
func (s *SessionService) CreateSession(input CreateSessionInput) (*models.Session, error) {
	if input.Courts < 1 || input.Courts > 3 {
		return nil, errors.New("courts must be between 1 and 3")
	}

	var session models.Session
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		club, err := lockClub(tx)
		if err != nil {
			return err
		}
		startsAt, _, err := utils.ResolveSessionTimes(input.SessionDate, input.StartTime, input.EndTime)
		if err != nil {
			return err
		}
		deadline := utils.CalculateRSVPDeadline(startsAt, club.RSVPDeadlineHours)
		if input.RSVPDeadline != nil {
			if err := validateDeadline(*input.RSVPDeadline, startsAt); err != nil {
				return err
			}
			deadline = *input.RSVPDeadline
		}
		session = models.Session{
			Title:              input.Title,
			Description:        input.Description,
			SessionDate:        input.SessionDate,
			StartTime:          input.StartTime,
			EndTime:            input.EndTime,
			Courts:             input.Courts,
			MaxPlayers:         models.MaxPlayersForCourts(input.Courts),
			RSVPDeadline:       deadline,
			IsRecurring:        input.IsRecurring,
			RecurringDayOfWeek: input.RecurringDayOfWeek,
			Status:             models.SessionStatusOpen,
			CreatedBy:          input.CreatedBy,
		}
		if input.IsRecurring {
			session.RecurringRSVPDeadline = input.RSVPDeadline
		}
		if err := tx.Create(&session).Error; err != nil {
			return err
		}
		if input.IsRecurring && input.RecurringDayOfWeek != nil {
			occurrences := 4
			if input.Occurrences != nil && *input.Occurrences > 0 {
				occurrences = *input.Occurrences
			}
			return s.generateRecurringSessions(tx, &session, occurrences, club.RSVPDeadlineHours)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func validateDeadline(deadline, startsAt time.Time) error {
	if deadline.Before(utils.NowInSydney()) {
		return errors.New("RSVP deadline cannot be in the past")
	}
	if deadline.After(startsAt) {
		return errors.New("RSVP deadline cannot be after the session starts")
	}
	return nil
}

// Unlike an explicit user edit, a stored series rule may refer to the past.
// It must still close RSVPs no later than the start of each generated session.
var errRecurringDeadline = errors.New("recurring RSVP deadline rule is invalid")

func validateRecurringDeadline(deadline, startsAt time.Time) error {
	if deadline.After(startsAt) {
		return fmt.Errorf("%w: session start cannot be before the recurring RSVP deadline. Change the club default to reset the recurring rule", errRecurringDeadline)
	}
	return nil
}

// generateRecurringSessions creates recurring session instances
func (s *SessionService) generateRecurringSessions(tx *gorm.DB, parent *models.Session, occurrences, defaultHours int) error {
	if parent.RecurringDayOfWeek == nil {
		return nil
	}

	// Derive the relative deadline offset from the parent session.
	// This preserves any custom deadline the admin set on the parent.
	parentDateSyd := parent.SessionDate.In(utils.SydneyLocation)
	parentDeadlineSyd := parent.RSVPDeadline.In(utils.SydneyLocation)
	if parent.RecurringRSVPDeadline != nil {
		parentDeadlineSyd = parent.RecurringRSVPDeadline.In(utils.SydneyLocation)
	}

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
		if err := tx.Model(&models.Session{}).
			Where("session_date = ? AND recurring_parent_id = ?", nextDate, parent.ID).
			Count(&count).Error; err != nil {
			return err
		}

		if count == 0 {
			// Generate title for this occurrence in format "Day - DD MMM YYYY"
			childTitle := nextDate.Format("Monday - 02 Jan 2006")

			startsAt, _, err := utils.ResolveSessionTimes(nextDate, parent.StartTime, parent.EndTime)
			if err != nil {
				return err
			}
			childDeadline := utils.CalculateRSVPDeadline(startsAt, defaultHours)
			if parent.RecurringRSVPDeadline != nil {
				// UTC is only a calendar container here, not the deadline instant.
				// Normalize the day offset before resolving the intended Sydney clock.
				childDateSyd := nextDate.In(utils.SydneyLocation)
				wallClock := time.Date(childDateSyd.Year(), childDateSyd.Month(), childDateSyd.Day()-daysBefore,
					deadlineHour, deadlineMin, deadlineSec, 0, time.UTC)
				childDeadline = time.Date(wallClock.Year(), wallClock.Month(), wallClock.Day(),
					deadlineHour, deadlineMin, deadlineSec, 0, utils.SydneyLocation)
				const clockFormat = "2006-01-02T15:04:05"
				if childDeadline.Format(clockFormat) != wallClock.Format(clockFormat) {
					return fmt.Errorf("%w: %s does not exist in Australia/Sydney. Choose another recurring deadline time", errRecurringDeadline, wallClock.Format(clockFormat))
				}
			}
			if err := validateRecurringDeadline(childDeadline, startsAt); err != nil {
				return err
			}

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
			if err := tx.Create(&child).Error; err != nil {
				return err
			}
		}

		nextDate = nextDate.AddDate(0, 0, 7)
	}

	return nil
}

// RefreshRecurringSessions generates any missing recurring session instances
// This is called for maintenance/refresh - uses default of 4 weeks ahead
func (s *SessionService) RefreshRecurringSessions() error {
	var invalidRules []error
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		club, err := lockClub(tx)
		if err != nil {
			return err
		}
		var parents []models.Session
		if err := tx.Where("is_recurring = ? AND status = ?", true, models.SessionStatusOpen).Find(&parents).Error; err != nil {
			return err
		}
		for i := range parents {
			// A savepoint keeps an invalid legacy series from rolling back
			// valid series. The club lock still protects the shared default.
			err := tx.Transaction(func(seriesTx *gorm.DB) error {
				return s.generateRecurringSessions(seriesTx, &parents[i], 4, club.RSVPDeadlineHours)
			})
			if errors.Is(err, errRecurringDeadline) {
				invalidRules = append(invalidRules, fmt.Errorf("recurring session %s: %w", parents[i].ID, err))
				continue
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return errors.Join(invalidRules...)
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
		club, err := lockClub(tx)
		if err != nil {
			return err
		}
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
			if session.RecurringRSVPDeadline != nil {
				oldDay := session.SessionDate.In(utils.SydneyLocation)
				newDay := input.SessionDate.In(utils.SydneyLocation)
				oldCalendarDay := time.Date(oldDay.Year(), oldDay.Month(), oldDay.Day(), 0, 0, 0, 0, time.UTC)
				newCalendarDay := time.Date(newDay.Year(), newDay.Month(), newDay.Day(), 0, 0, 0, 0, time.UTC)
				days := int(newCalendarDay.Sub(oldCalendarDay).Hours() / 24)
				shifted := session.RecurringRSVPDeadline.In(utils.SydneyLocation).AddDate(0, 0, days)
				session.RecurringRSVPDeadline = &shifted
			}
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

		if input.RSVPDeadline != nil || input.SessionDate != nil || input.StartTime != nil {
			startsAt, _, err := utils.ResolveSessionTimes(session.SessionDate, session.StartTime, session.EndTime)
			if err != nil {
				return err
			}
			if session.IsRecurring && session.RecurringRSVPDeadline != nil {
				if err := validateRecurringDeadline(*session.RecurringRSVPDeadline, startsAt); err != nil {
					return err
				}
			}
			if input.RSVPDeadline != nil {
				if err := validateDeadline(*input.RSVPDeadline, startsAt); err != nil {
					return err
				}
				session.RSVPDeadline = *input.RSVPDeadline
			} else {
				session.RSVPDeadline = utils.CalculateRSVPDeadline(startsAt, club.RSVPDeadlineHours)
			}
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
