package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/weekday-masters/backend/internal/config"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/handlers"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/services"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Set Gin mode
	gin.SetMode(cfg.GinMode)

	// Connect to database
	if err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Run migrations
	if err := database.Migrate(); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	// Initialize services
	userService := services.NewUserService(cfg.AdminEmail)
	sessionService := services.NewSessionService()
	rsvpService := services.NewRSVPService()

	// Initialize notification service
	notificationService := services.NewNotificationService(services.NotificationConfig{
		FirebaseCredentials: cfg.FirebaseCredentials,
		SendGridAPIKey:      cfg.SendGridAPIKey,
		SendGridFromEmail:   cfg.SendGridFromEmail,
		SendGridFromName:    cfg.SendGridFromName,
		FrontendURL:         cfg.FrontendURL,
	})

	// Initialize scheduler for notification cron jobs
	var scheduler *services.SchedulerService
	if notificationService.IsEnabled() {
		scheduler = services.NewSchedulerService(services.SchedulerConfig{
			NotificationService:    notificationService,
			SessionReminderHours24: cfg.SessionReminderHours24,
			SessionReminderHours12: cfg.SessionReminderHours12,
			DeadlineReminderHours:  cfg.DeadlineReminderHours,
		})
		scheduler.Start()
	}

	// Refresh recurring sessions on startup
	if err := sessionService.RefreshRecurringSessions(); err != nil {
		log.Println("Warning: Failed to refresh recurring sessions:", err)
	}

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(userService)
	userHandler := handlers.NewUserHandler(userService)
	sessionHandler := handlers.NewSessionHandler(sessionService, rsvpService)
	rsvpHandler := handlers.NewRSVPHandler(rsvpService)
	adminHandler := handlers.NewAdminHandler(userService, sessionService, rsvpService)
	notificationHandler := handlers.NewNotificationHandler(notificationService)

	// Auth0 config for middleware
	auth0Config := middleware.Auth0Config{
		Domain:   cfg.Auth0Domain,
		Audience: cfg.Auth0Audience,
	}

	// Setup router
	r := gin.Default()

	// CORS middleware
	r.Use(middleware.CORS(cfg.FrontendURL))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API routes
	api := r.Group("/api")
	{
		// Public routes
		api.POST("/auth/callback", authHandler.Callback)
		api.GET("/club", adminHandler.GetClub)

		// Protected routes (requires valid JWT)
		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(auth0Config))
		{
			// User routes
			protected.GET("/users/me", userHandler.GetMe)
			protected.PUT("/users/me", userHandler.UpdateMe)

			// Notification preferences routes (available to all authenticated users)
			protected.GET("/users/me/notifications", notificationHandler.GetPreferences)
			protected.PUT("/users/me/notifications", notificationHandler.UpdatePreferences)
			protected.POST("/users/me/push-tokens", notificationHandler.RegisterPushToken)
			protected.DELETE("/users/me/push-tokens", notificationHandler.UnregisterPushToken)
			protected.GET("/users/me/notifications/history", notificationHandler.GetNotificationHistory)
			protected.POST("/notifications/:id/read", notificationHandler.MarkNotificationRead)

			// These routes require approved membership
			approved := protected.Group("")
			approved.Use(middleware.RequireApproved())
			{
				protected.GET("/users", userHandler.ListMembers)

				// Session routes
				protected.GET("/sessions", sessionHandler.ListSessions)
				protected.GET("/sessions/cancelled", sessionHandler.ListCancelledSessions)
				protected.GET("/sessions/:id", sessionHandler.GetSession)

				// RSVP routes
				protected.POST("/sessions/:id/rsvp", rsvpHandler.CreateRSVP)
				protected.PUT("/sessions/:id/rsvp", rsvpHandler.UpdateRSVP)
				protected.DELETE("/sessions/:id/rsvp", rsvpHandler.DeleteRSVP)
				protected.GET("/sessions/:id/rsvp/me", rsvpHandler.GetMyRSVP)
			}

			// Admin routes
			admin := protected.Group("/admin")
			admin.Use(middleware.RequireAdmin())
			{
				// Join requests
				admin.GET("/join-requests", adminHandler.ListJoinRequests)
				admin.POST("/join-requests/:id/approve", adminHandler.ApproveJoinRequest)
				admin.POST("/join-requests/:id/reject", adminHandler.RejectJoinRequest)

				// User management
				admin.PUT("/users/:id/role", adminHandler.UpdateUserRole)

				// Session management
				admin.POST("/sessions", adminHandler.CreateSession)
				admin.PUT("/sessions/:id", adminHandler.UpdateSession)
				admin.DELETE("/sessions/:id", adminHandler.DeleteSession)
				admin.POST("/sessions/:id/cancel", adminHandler.CancelSession)

				// Admin RSVP management
				admin.POST("/sessions/:id/rsvp/:userId", adminHandler.AddPlayerRSVP)

				// Club management
				admin.PUT("/club", adminHandler.UpdateClub)

				// Announcements
				admin.POST("/announcements", notificationHandler.SendAnnouncement)
			}
		}
	}

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		log.Printf("Server starting on port %s", cfg.Port)
		if err := r.Run(":" + cfg.Port); err != nil {
			log.Fatal("Failed to start server:", err)
		}
	}()

	// Wait for shutdown signal
	<-quit
	log.Println("Shutting down server...")

	// Stop scheduler if running
	if scheduler != nil {
		scheduler.Stop()
	}

	log.Println("Server stopped")
}
