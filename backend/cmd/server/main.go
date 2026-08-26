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

	// Connect to database. Schema migrations are NOT run here — they are applied by
	// the separate `migrate` command as an explicit deploy step (see cmd/migrate).
	if err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Initialize notification service first: the RSVP service uses it to tell players
	// when they have been promoted off a session waitlist.
	notificationService := services.NewNotificationService(services.NotificationConfig{
		FirebaseCredentials: cfg.FirebaseCredentials,
		SendGridAPIKey:      cfg.SendGridAPIKey,
		SendGridFromEmail:   cfg.SendGridFromEmail,
		SendGridFromName:    cfg.SendGridFromName,
		FrontendURL:         cfg.FrontendURL,
	})

	// Initialize services
	userService := services.NewUserService(cfg.AdminEmail)
	auth0Service := services.NewAuth0Service(cfg.Auth0Domain)
	sessionService := services.NewSessionService()
	rsvpService := services.NewRSVPService(notificationService)

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
	authHandler := handlers.NewAuthHandler(userService, auth0Service)
	userHandler := handlers.NewUserHandler(userService)
	sessionHandler := handlers.NewSessionHandler(sessionService, rsvpService)
	rsvpHandler := handlers.NewRSVPHandler(rsvpService)
	adminHandler := handlers.NewAdminHandler(userService, sessionService, rsvpService)
	notificationHandler := handlers.NewNotificationHandler(notificationService)
	ledgerService := services.NewLedgerService()
	ledgerHandler := handlers.NewLedgerHandler(ledgerService)

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
		api.GET("/openapi", handlers.RedirectOpenAPI)
		api.GET("/openapi/index.html", handlers.ServeOpenAPIIndex)
		api.GET("/openapi/spec.yaml", handlers.ServeOpenAPISpec)
		api.GET("/club", adminHandler.GetClub)

		// Registration: requires a valid Auth0 token, but not an existing user row.
		// Identity is read from the verified token, not the request body.
		registration := api.Group("")
		registration.Use(middleware.RequireValidToken(auth0Config))
		registration.POST("/auth/callback", authHandler.Callback)

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
				approved.GET("/users", userHandler.ListMembers)

				// Session routes
				approved.GET("/sessions", sessionHandler.ListSessions)
				approved.GET("/sessions/cancelled", sessionHandler.ListCancelledSessions)
				approved.GET("/sessions/:id", sessionHandler.GetSession)

				// RSVP routes
				approved.POST("/sessions/:id/rsvp", rsvpHandler.CreateRSVP)
				approved.PUT("/sessions/:id/rsvp", rsvpHandler.UpdateRSVP)
				approved.DELETE("/sessions/:id/rsvp", rsvpHandler.DeleteRSVP)
				approved.GET("/sessions/:id/rsvp/me", rsvpHandler.GetMyRSVP)

				// Ledger reads. Every approved member can see every balance:
				// the club already worked this way in Splitwise.
				approved.GET("/accounts", ledgerHandler.ListBalances)
				approved.GET("/accounts/me", ledgerHandler.GetMyBalance)
				approved.GET("/accounts/me/entries", ledgerHandler.GetMyEntries)
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

				// Ledger writes. Only admins move money; there is deliberately
				// no edit or delete route, only reversal.
				admin.POST("/transactions/topup", ledgerHandler.RecordTopup)
				admin.POST("/transactions/withdrawal", ledgerHandler.RecordWithdrawal)
				admin.POST("/transactions/court-credit", ledgerHandler.RecordCourtCredit)
				admin.POST("/transactions/shuttle-purchase", ledgerHandler.RecordShuttlePurchase)
				admin.POST("/transactions/opening-balances", ledgerHandler.RecordOpeningBalances)
				admin.POST("/transactions/:id/reverse", ledgerHandler.ReverseTransaction)

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
