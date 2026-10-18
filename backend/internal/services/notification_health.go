package services

import (
	"context"
	"errors"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"time"
)

type NotificationWorkHealth struct {
	Overdue int64 `json:"overdue"`
}

// CheckNotificationWorkHealth is invoked by an independent Scheduler job. It
// reads durable backlog even when the drain fails or its provider is unavailable.
func CheckNotificationWorkHealth(ctx context.Context, now time.Time) (*NotificationWorkHealth, error) {
	result := &NotificationWorkHealth{}
	err := database.DB.WithContext(ctx).Model(&models.NotificationOutbox{}).
		Where("state IN ? AND due_at <= ? AND next_attempt_at <= ?", []models.DeliveryState{models.DeliveryPending, models.DeliveryClaimed}, now, now.Add(-5*time.Minute)).Count(&result.Overdue).Error
	if err != nil {
		return nil, err
	}
	if result.Overdue > 0 {
		return result, errors.New("notification backlog overdue")
	}
	return result, nil
}
