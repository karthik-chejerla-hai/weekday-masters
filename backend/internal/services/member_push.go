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
		Where("user_id = ? AND revoked_at IS NULL AND invalidated_at IS NULL", id).Order("last_used_at DESC, id").Scan(&result.Devices).Error
	if err != nil {
		return nil, err
	}
	return result, nil
}

// AdminMember includes a registration summary, not browser permission or delivery status.
type AdminMember struct {
	models.User
	PushDeviceCount int64 `json:"push_device_count"`
}

// ListMembersWithPushSummary uses one grouped query for the complete roll. Errors
// fail the list rather than misrepresent unavailable registration data as zero.
func (s *UserService) ListMembersWithPushSummary() ([]AdminMember, error) {
	users, err := s.ListAllMembers()
	if err != nil {
		return nil, err
	}
	result := make([]AdminMember, len(users))
	if len(users) == 0 {
		return result, nil
	}
	ids := make([]uuid.UUID, len(users))
	for i, user := range users {
		ids[i] = user.ID
	}
	var counts []struct {
		UserID      uuid.UUID
		DeviceCount int64
	}
	if err := database.DB.Model(&models.UserPushToken{}).
		Select("user_id, COUNT(*) AS device_count").Where("user_id IN ? AND revoked_at IS NULL AND invalidated_at IS NULL", ids).
		Group("user_id").Scan(&counts).Error; err != nil {
		return nil, err
	}
	byUser := make(map[uuid.UUID]int64, len(counts))
	for _, count := range counts {
		byUser[count.UserID] = count.DeviceCount
	}
	for i, user := range users {
		result[i] = AdminMember{User: user, PushDeviceCount: byUser[user.ID]}
	}
	return result, nil
}
