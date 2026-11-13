package services

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
)

// NotificationDeliveryStatus exposes only aggregate outcomes, never device
// identifiers, addresses, proofs, provider errors or notification contents.
type NotificationDeliveryStatus struct {
	NotificationID uuid.UUID                      `json:"notification_id"`
	State          models.DeliveryState           `json:"state"`
	Devices        map[models.DeliveryState]int64 `json:"devices"`
}

func (s *NotificationService) GetDeliveryStatus(ctx context.Context, id uuid.UUID) (*NotificationDeliveryStatus, error) {
	result := &NotificationDeliveryStatus{NotificationID: id, Devices: map[models.DeliveryState]int64{}}
	err := database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var outbox models.NotificationOutbox
		if err := tx.Where("notification_id = ?", id).First(&outbox).Error; err != nil {
			return err
		}
		result.State = outbox.State
		var counts []struct {
			State models.DeliveryState
			Count int64
		}
		if err := tx.Model(&models.NotificationDeliveryAttempt{}).Select("state, COUNT(*) AS count").Where("outbox_id = ?", outbox.ID).Group("state").Scan(&counts).Error; err != nil {
			return err
		}
		for _, c := range counts {
			result.Devices[c.State] = c.Count
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return result, nil
}
