package services

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
)

type GuestRSVPInput struct {
	ID              uuid.UUID
	SessionID       uuid.UUID
	GuestName       string
	ChargedToUserID uuid.UUID
	Status          models.RSVPStatus
}

// SaveGuestRSVP shares the member RSVP lock, so a guest and a member cannot
// claim the same last place. Guests fill vacancies; admins cannot overbook them.
func (s *RSVPService) SaveGuestRSVP(in GuestRSVPInput) (*models.GuestRSVP, error) {
	in.GuestName = strings.Join(strings.Fields(in.GuestName), " ")
	if in.GuestName == "" || utf8.RuneCountInString(in.GuestName) > 255 {
		return nil, errors.New("enter a guest name of 1 to 255 characters")
	}
	if in.ChargedToUserID == uuid.Nil {
		return nil, errors.New("select an approved member to pay for the guest")
	}
	if in.Status != models.RSVPStatusIn && in.Status != models.RSVPStatusOut {
		return nil, errors.New("guest response must be IN or OUT")
	}
	var guest models.GuestRSVP
	err := s.withRSVPLocks(in.SessionID, in.ChargedToUserID, func(tx *gorm.DB, session models.Session) ([]uuid.UUID, error) {
		if err := validateGuestRSVPChange(tx, session); err != nil {
			return nil, err
		}
		if in.ID != uuid.Nil {
			if err := tx.Where("id = ? AND session_id = ?", in.ID, in.SessionID).First(&guest).Error; err != nil {
				return nil, errors.New("guest RSVP not found")
			}
		}
		// The session lock serializes both creates and edits. A payer or status
		// change must not give the same named guest a second place or charge.
		var existing []models.GuestRSVP
		if err := tx.Where("session_id = ? AND id <> ?", in.SessionID, in.ID).Find(&existing).Error; err != nil {
			return nil, err
		}
		for _, other := range existing {
			if strings.EqualFold(strings.Join(strings.Fields(other.GuestName), " "), in.GuestName) {
				return nil, fmt.Errorf("this session already has a guest named %s. Edit their RSVP instead", other.GuestName)
			}
		}
		wasIn := guest.Status == models.RSVPStatusIn
		if in.Status == models.RSVPStatusIn && !wasIn {
			confirmed, err := countConfirmed(tx, session.ID)
			if err != nil {
				return nil, err
			}
			if confirmed >= session.MaxPlayers {
				return nil, errors.New("this session is full; a guest needs a vacant place")
			}
		}
		guest.SessionID = in.SessionID
		guest.GuestName = in.GuestName
		guest.ChargedToUserID = in.ChargedToUserID
		guest.Status = in.Status
		if err := tx.Save(&guest).Error; err != nil {
			return nil, err
		}
		if wasIn && in.Status == models.RSVPStatusOut {
			return promoteWithinTx(tx, session)
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}
	if err := database.DB.Preload("ChargedToUser").First(&guest, "id = ?", guest.ID).Error; err != nil {
		return nil, err
	}
	return &guest, nil
}

// DeleteGuestRSVP removes an unplayed guest booking, including its payer
// obligation. The session lock also protects the place released to the waitlist.
func (s *RSVPService) DeleteGuestRSVP(sessionID, guestID uuid.UUID) error {
	return s.withSessionLock(sessionID, func(tx *gorm.DB, session models.Session) ([]uuid.UUID, error) {
		if err := validateGuestRSVPChange(tx, session); err != nil {
			return nil, err
		}
		var guest models.GuestRSVP
		if err := tx.Where("id = ? AND session_id = ?", guestID, sessionID).First(&guest).Error; err != nil {
			return nil, errors.New("guest RSVP not found")
		}
		if err := tx.Delete(&guest).Error; err != nil {
			return nil, err
		}
		if guest.Status == models.RSVPStatusIn {
			return promoteWithinTx(tx, session)
		}
		return nil, nil
	})
}

// Call only with the session locked, so deletion cannot erase settled history.
func validateGuestRSVPChange(tx *gorm.DB, session models.Session) error {
	if session.Status != models.SessionStatusOpen || session.StartsAt == nil || !session.StartsAt.After(utils.NowInSydney()) {
		return errors.New("guests can only be changed for an upcoming open session")
	}
	var settled int64
	if err := tx.Model(&models.Settlement{}).Where("session_id = ?", session.ID).Count(&settled).Error; err != nil {
		return err
	}
	if settled > 0 {
		return errors.New("guests cannot be changed for a settled session")
	}
	return nil
}
