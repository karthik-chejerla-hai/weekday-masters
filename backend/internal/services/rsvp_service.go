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

type RSVPService struct{}

func NewRSVPService() *RSVPService {
	return &RSVPService{}
}

type RSVPInput struct {
	SessionID uuid.UUID
	UserID    uuid.UUID
	Status    models.RSVPStatus
}

// CreateOrUpdateRSVP creates or updates an RSVP
func (s *RSVPService) CreateOrUpdateRSVP(input RSVPInput, byAdmin bool) (*models.RSVP, error) {
	// Get the session
	var session models.Session
	if err := database.DB.First(&session, "id = ?", input.SessionID).Error; err != nil {
		return nil, errors.New("session not found")
	}

	// Check if session is open
	if session.Status != models.SessionStatusOpen {
		return nil, errors.New("session is not open for RSVPs")
	}

	now := utils.NowInSydney()
	isLate := now.After(session.RSVPDeadline)

	// Check RSVP deadline for non-admin
	if !byAdmin && isLate {
		return nil, errors.New("RSVP deadline has passed")
	}

	// Check if RSVP already exists
	var rsvp models.RSVP
	result := database.DB.Where("session_id = ? AND user_id = ?", input.SessionID, input.UserID).First(&rsvp)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Create new RSVP
			rsvp = models.RSVP{
				SessionID:     input.SessionID,
				UserID:        input.UserID,
				Status:        input.Status,
				RSVPTimestamp: now,
				IsLateRSVP:    isLate,
				AddedByAdmin:  byAdmin,
			}

			if err := database.DB.Create(&rsvp).Error; err != nil {
				return nil, err
			}
		} else {
			return nil, result.Error
		}
	} else {
		// Check if user is trying to change from IN to OUT after deadline
		if !byAdmin && isLate && rsvp.Status == models.RSVPStatusIn && input.Status != models.RSVPStatusIn {
			return nil, errors.New("cannot change RSVP from IN after deadline")
		}

		// Update existing RSVP
		rsvp.Status = input.Status
		rsvp.UpdatedAt = time.Now()

		// Don't update timestamp unless admin is changing it
		if byAdmin {
			rsvp.AddedByAdmin = true
		}

		if err := database.DB.Save(&rsvp).Error; err != nil {
			return nil, err
		}
	}

	// Load user details
	database.DB.Preload("User").First(&rsvp, "id = ?", rsvp.ID)

	return &rsvp, nil
}

// DeleteRSVP removes an RSVP
func (s *RSVPService) DeleteRSVP(sessionID, userID uuid.UUID, byAdmin bool) error {
	// Get the session
	var session models.Session
	if err := database.DB.First(&session, "id = ?", sessionID).Error; err != nil {
		return errors.New("session not found")
	}

	// Get the RSVP
	var rsvp models.RSVP
	if err := database.DB.Where("session_id = ? AND user_id = ?", sessionID, userID).First(&rsvp).Error; err != nil {
		return errors.New("RSVP not found")
	}

	now := utils.NowInSydney()
	isLate := now.After(session.RSVPDeadline)

	// Check if user is trying to delete IN RSVP after deadline
	if !byAdmin && isLate && rsvp.Status == models.RSVPStatusIn {
		return errors.New("cannot remove IN RSVP after deadline")
	}

	return database.DB.Delete(&rsvp).Error
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
	return rsvps, nil
}

// GetUserRSVPForSession returns a user's RSVP for a session
func (s *RSVPService) GetUserRSVPForSession(sessionID, userID uuid.UUID) (*models.RSVP, error) {
	var rsvp models.RSVP
	if err := database.DB.Where("session_id = ? AND user_id = ?", sessionID, userID).
		First(&rsvp).Error; err != nil {
		return nil, err
	}
	return &rsvp, nil
}

// RSVPSummary contains summary statistics for a session's RSVPs
type RSVPSummary struct {
	TotalIn    int `json:"total_in"`
	TotalOut   int `json:"total_out"`
	TotalMaybe int `json:"total_maybe"`
	MaxPlayers int `json:"max_players"`
	SpotsLeft  int `json:"spots_left"`
}

// GetRSVPSummary returns summary statistics for a session
func (s *RSVPService) GetRSVPSummary(sessionID uuid.UUID) (*RSVPSummary, error) {
	var session models.Session
	if err := database.DB.First(&session, "id = ?", sessionID).Error; err != nil {
		return nil, err
	}

	var inCount, outCount, maybeCount int64

	database.DB.Model(&models.RSVP{}).
		Where("session_id = ? AND status = ?", sessionID, models.RSVPStatusIn).
		Count(&inCount)

	database.DB.Model(&models.RSVP{}).
		Where("session_id = ? AND status = ?", sessionID, models.RSVPStatusOut).
		Count(&outCount)

	database.DB.Model(&models.RSVP{}).
		Where("session_id = ? AND status = ?", sessionID, models.RSVPStatusMaybe).
		Count(&maybeCount)

	spotsLeft := session.MaxPlayers - int(inCount)
	if spotsLeft < 0 {
		spotsLeft = 0
	}

	return &RSVPSummary{
		TotalIn:    int(inCount),
		TotalOut:   int(outCount),
		TotalMaybe: int(maybeCount),
		MaxPlayers: session.MaxPlayers,
		SpotsLeft:  spotsLeft,
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
