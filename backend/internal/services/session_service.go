package services

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
)

type SessionService struct{}

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
}

// CreateSession creates a new session
func (s *SessionService) CreateSession(input CreateSessionInput) (*models.Session, error) {
	if input.Courts < 1 || input.Courts > 3 {
		return nil, errors.New("courts must be between 1 and 3")
	}

	session := models.Session{
		Title:              input.Title,
		Description:        input.Description,
		SessionDate:        input.SessionDate,
		StartTime:          input.StartTime,
		EndTime:            input.EndTime,
		Courts:             input.Courts,
		MaxPlayers:         models.MaxPlayersForCourts(input.Courts),
		RSVPDeadline:       utils.CalculateRSVPDeadline(input.SessionDate),
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

			child := models.Session{
				Title:             childTitle,
				Description:       parent.Description,
				SessionDate:       nextDate,
				StartTime:         parent.StartTime,
				EndTime:           parent.EndTime,
				Courts:            parent.Courts,
				MaxPlayers:        parent.MaxPlayers,
				RSVPDeadline:      utils.CalculateRSVPDeadline(nextDate),
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

	if err := database.DB.Where("session_date >= ? AND status != ?", today, models.SessionStatusCancelled).
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

	if err := database.DB.Where("session_date >= ? AND status = ?", today, models.SessionStatusCancelled).
		Order("session_date ASC, start_time ASC").
		Find(&sessions).Error; err != nil {
		return nil, err
	}

	return sessions, nil
}

type UpdateSessionInput struct {
	Title       *string
	Description *string
	SessionDate *time.Time
	StartTime   *string
	EndTime     *string
	Courts      *int
	Status      *models.SessionStatus
}

// UpdateSession updates a session
func (s *SessionService) UpdateSession(id uuid.UUID, input UpdateSessionInput) (*models.Session, error) {
	var session models.Session
	if err := database.DB.First(&session, "id = ?", id).Error; err != nil {
		return nil, err
	}

	if input.Title != nil {
		session.Title = *input.Title
	}
	if input.Description != nil {
		session.Description = *input.Description
	}
	if input.SessionDate != nil {
		session.SessionDate = *input.SessionDate
		session.RSVPDeadline = utils.CalculateRSVPDeadline(*input.SessionDate)
	}
	if input.StartTime != nil {
		session.StartTime = *input.StartTime
	}
	if input.EndTime != nil {
		session.EndTime = *input.EndTime
	}
	if input.Courts != nil {
		if *input.Courts < 1 || *input.Courts > 3 {
			return nil, errors.New("courts must be between 1 and 3")
		}
		session.Courts = *input.Courts
		session.MaxPlayers = models.MaxPlayersForCourts(*input.Courts)
	}
	if input.Status != nil {
		session.Status = *input.Status
	}

	session.UpdatedAt = time.Now()

	if err := database.DB.Save(&session).Error; err != nil {
		return nil, err
	}

	return &session, nil
}

// DeleteSession deletes or cancels a session
func (s *SessionService) DeleteSession(id uuid.UUID) error {
	var session models.Session
	if err := database.DB.First(&session, "id = ?", id).Error; err != nil {
		return err
	}

	// If session has RSVPs, just mark as cancelled
	var rsvpCount int64
	database.DB.Model(&models.RSVP{}).Where("session_id = ?", id).Count(&rsvpCount)

	if rsvpCount > 0 {
		session.Status = models.SessionStatusCancelled
		session.UpdatedAt = time.Now()
		return database.DB.Save(&session).Error
	}

	// Otherwise, delete it
	return database.DB.Delete(&session).Error
}

// CancelSession cancels a session with an optional reason
func (s *SessionService) CancelSession(id uuid.UUID, reason string) (*models.Session, error) {
	var session models.Session
	if err := database.DB.First(&session, "id = ?", id).Error; err != nil {
		return nil, err
	}

	session.Status = models.SessionStatusCancelled
	session.CancellationReason = reason
	session.UpdatedAt = time.Now()

	if err := database.DB.Save(&session).Error; err != nil {
		return nil, err
	}

	return &session, nil
}
