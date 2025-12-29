package database

import (
	"log"

	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Connect(databaseURL string) error {
	var err error
	DB, err = gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return err
	}

	log.Println("Connected to database")
	return nil
}

func Migrate() error {
	log.Println("Running database migrations...")

	err := DB.AutoMigrate(
		&models.Club{},
		&models.User{},
		&models.Session{},
		&models.RSVP{},
		// Notification models
		&models.UserNotificationPreferences{},
		&models.UserPushToken{},
		&models.Notification{},
		&models.Announcement{},
	)
	if err != nil {
		return err
	}

	// Seed default club if not exists
	var count int64
	DB.Model(&models.Club{}).Count(&count)
	if count == 0 {
		club := models.Club{
			Name: "Weekday Masters Badminton Club",
		}
		DB.Create(&club)
		log.Println("Created default club")
	}

	log.Println("Database migrations completed")
	return nil
}
