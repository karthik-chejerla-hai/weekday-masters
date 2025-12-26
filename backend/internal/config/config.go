package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         string
	DatabaseURL  string
	Auth0Domain  string
	Auth0Audience string
	AdminEmail   string
	Timezone     string
	FrontendURL  string
	GinMode      string
}

func Load() *Config {
	godotenv.Load()

	return &Config{
		Port:         getEnv("PORT", "8080"),
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://badminton:badminton123@localhost:5432/badminton_club?sslmode=disable"),
		Auth0Domain:  getEnv("AUTH0_DOMAIN", ""),
		Auth0Audience: getEnv("AUTH0_AUDIENCE", ""),
		AdminEmail:   getEnv("ADMIN_EMAIL", ""),
		Timezone:     getEnv("TIMEZONE", "Australia/Sydney"),
		FrontendURL:  getEnv("FRONTEND_URL", "http://localhost:5173"),
		GinMode:      getEnv("GIN_MODE", "debug"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
