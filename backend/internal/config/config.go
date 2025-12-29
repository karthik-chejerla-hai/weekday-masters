package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port          string
	DatabaseURL   string
	Auth0Domain   string
	Auth0Audience string
	AdminEmail    string
	Timezone      string
	FrontendURL   string
	GinMode       string

	// Firebase FCM configuration
	FirebaseProjectID   string
	FirebaseCredentials string // JSON string of service account credentials

	// SendGrid email configuration
	SendGridAPIKey    string
	SendGridFromEmail string
	SendGridFromName  string

	// Notification timing settings (in hours)
	SessionReminderHours24 int // First reminder (default 24h before)
	SessionReminderHours12 int // Second reminder (default 12h before)
	DeadlineReminderHours  int // RSVP deadline alert (default 6h before)
}

func Load() *Config {
	godotenv.Load()

	return &Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://badminton:badminton123@localhost:5432/badminton_club?sslmode=disable"),
		Auth0Domain:   getEnv("AUTH0_DOMAIN", ""),
		Auth0Audience: getEnv("AUTH0_AUDIENCE", ""),
		AdminEmail:    getEnv("ADMIN_EMAIL", ""),
		Timezone:      getEnv("TIMEZONE", "Australia/Sydney"),
		FrontendURL:   getEnv("FRONTEND_URL", "http://localhost:5173"),
		GinMode:       getEnv("GIN_MODE", "debug"),

		// Firebase FCM
		FirebaseProjectID:   getEnv("FIREBASE_PROJECT_ID", ""),
		FirebaseCredentials: getEnv("FIREBASE_CREDENTIALS", ""),

		// SendGrid
		SendGridAPIKey:    getEnv("SENDGRID_API_KEY", ""),
		SendGridFromEmail: getEnv("SENDGRID_FROM_EMAIL", "noreply@weekdaymasters.club"),
		SendGridFromName:  getEnv("SENDGRID_FROM_NAME", "Weekday Masters"),

		// Notification timing
		SessionReminderHours24: getEnvInt("SESSION_REMINDER_HOURS_24", 24),
		SessionReminderHours12: getEnvInt("SESSION_REMINDER_HOURS_12", 12),
		DeadlineReminderHours:  getEnvInt("DEADLINE_REMINDER_HOURS", 6),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
