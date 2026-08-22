// Command migrate applies the database schema and exits.
//
// It is deliberately separate from the API server: running migrations as a side
// effect of process start means every cold start and every concurrent revision
// races to mutate the schema. Run this once as an explicit deploy step, before
// rolling out a new server revision.
package main

import (
	"log"

	"github.com/weekday-masters/backend/internal/config"
	"github.com/weekday-masters/backend/internal/database"
)

func main() {
	cfg := config.Load()

	if err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	if err := database.Migrate(); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	log.Println("Migrations applied successfully")
}
