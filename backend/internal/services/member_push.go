package services

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
)

// PushDevice exposes registration metadata, never the delivery token.
type PushDevice struct {
	ID               uuid.UUID `json:"id"`
	DeviceName       string    `json:"device_name"`
	CreatedAt        time.Time `json:"created_at"`
	LastRegisteredAt time.Time `json:"last_registered_at"`
}

type MemberPushStatus struct {
	Preferences *models.UserNotificationPreferences `json:"preferences"`
	Devices     []PushDevice                        `json:"devices"`
}

// MemberPushStatus is read-only. A missing preference row is not proof of consent.
func (s *UserService) MemberPushStatus(id uuid.UUID) (*MemberPushStatus, error) {
	if _, err := s.GetUserByID(id); err != nil {
		return nil, err
	}
	result := &MemberPushStatus{Devices: []PushDevice{}}
	var prefs models.UserNotificationPreferences
	err := database.DB.Where("user_id = ?", id).First(&prefs).Error
	if err == nil {
		result.Preferences = &prefs
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	err = database.DB.Model(&models.UserPushToken{}).
		Select("id, device_name, created_at, last_used_at AS last_registered_at").
		Where("user_id = ?", id).Order("last_used_at DESC, id").Scan(&result.Devices).Error
	if err != nil {
		return nil, err
	}
	return result, nil
}
