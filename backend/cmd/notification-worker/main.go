// notification-worker serves only private scheduled notification work. It never
// migrates the schema and never mounts member/admin or money-writing routes.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/weekday-masters/backend/internal/config"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/handlers"
	"github.com/weekday-masters/backend/internal/services"
	"google.golang.org/api/idtoken"
)

func main() {
	cfg := config.Load()
	audience := os.Getenv("NOTIFICATION_WORKER_AUDIENCE")
	schedulerEmail := os.Getenv("NOTIFICATION_SCHEDULER_EMAIL")
	if audience == "" || schedulerEmail == "" {
		log.Fatal("Notification worker audience and scheduler identity are required")
	}
	if err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatal("Notification worker database connection failed")
	}
	sqlDB, err := database.DB.DB()
	if err != nil {
		log.Fatal("Notification worker database unavailable")
	}
	defer sqlDB.Close()
	notifications := services.NewNotificationService(services.NotificationConfig{
		Disabled: cfg.NotificationsDisabled, FirebaseProjectID: cfg.FirebaseProjectID, FirebaseCredentials: cfg.FirebaseCredentials, FrontendURL: cfg.FrontendURL,
	})
	// Preview worker does not inherit the API's diagnostic email allowlist.
	router := handlers.NewNotificationWorkerRouter(audience, schedulerEmail, idtoken.Validate, func(ctx context.Context) (any, error) {
		return notifications.ProcessNotificationWork(ctx, 100)
	})
	server := &http.Server{Addr: ":" + cfg.Port, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Notification worker HTTP server failed")
		}
	}()
	<-done
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Print("Notification worker shutdown timed out")
	}
}
