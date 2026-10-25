package database

import (
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"time"
)

// MigrateNotificationDeliverySchema is shared by the CLI and scratch harness.
func MigrateNotificationDeliverySchema(db *gorm.DB) error {
	return db.AutoMigrate(&models.NotificationMigrationRecord{}, &models.NotificationOutbox{}, &models.NotificationDeliveryAttempt{})
}

// RunNotificationMigration serializes a version and captures one rollout cutoff.
// A failure rolls back both its work and completion; a repeated successful run
// does nothing. Call after MigrateNotificationDeliverySchema from database.Migrate.
func RunNotificationMigration(db *gorm.DB, version string, apply func(*gorm.DB, time.Time) error) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "notification-migration:"+version).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.NotificationMigrationRecord{}).Where("version = ?", version).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return nil
		}
		cutoff := time.Now().UTC()
		if err := apply(tx, cutoff); err != nil {
			return err
		}
		return tx.Create(&models.NotificationMigrationRecord{Version: version, Cutoff: cutoff, CompletedAt: time.Now().UTC()}).Error
	})
}
