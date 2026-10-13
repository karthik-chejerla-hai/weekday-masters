package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/weekday-masters/backend/internal/assistant"
	"github.com/weekday-masters/backend/internal/config"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/handlers"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/services"
	"github.com/weekday-masters/backend/internal/telemetry"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() (runErr error) {
	// Load configuration
	cfg := config.Load()
	if err := cfg.ValidateNotificationDeliveryReadiness(); err != nil {
		return err
	}
	obs, err := telemetry.Start(context.Background())
	if err != nil {
		return fmt.Errorf("initialize telemetry: %w", err)
	}
	defer func() {
		if runErr != nil {
			log.Printf("Server failed: %v", runErr)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		if err := obs.Shutdown(ctx); err != nil {
			log.Printf("flush telemetry: %v", err)
		}
	}()

	// Set Gin mode
	gin.SetMode(cfg.GinMode)

	// Connect to database. Schema migrations are NOT run here — they are applied by
	// the separate `migrate` command as an explicit deploy step (see cmd/migrate).
	if err := database.Connect(cfg.DatabaseURL); err != nil {
		return fmt.Errorf("connect database: %w", err)
	}

	sqlDB, err := database.DB.DB()
	if err != nil {
		return err
	}
	if err := obs.ObserveDB(sqlDB); err != nil {
		return err
	}
	defer sqlDB.Close()

	// Initialize notifications independently of RSVP capacity and waitlist promotion.
	notificationService := services.NewNotificationService(services.NotificationConfig{
		Disabled:            cfg.NotificationsDisabled,
		DisabledAllowEmail:  cfg.NotificationsDisabledAllowEmail,
		FirebaseCredentials: cfg.FirebaseCredentials,
		FirebaseProjectID:   cfg.FirebaseProjectID,
		FrontendURL:         cfg.FrontendURL,
	})
	invitationService := services.NewInvitationService(services.NotificationConfig{
		Disabled:          cfg.NotificationsDisabled,
		SendGridAPIKey:    cfg.SendGridAPIKey,
		SendGridFromEmail: cfg.SendGridFromEmail,
		SendGridFromName:  cfg.SendGridFromName,
		FrontendURL:       cfg.FrontendURL,
	}, cfg.InvitationTestEmailsEnabled)
	invitationHandler := handlers.NewInvitationHandler(invitationService)

	// Initialize services
	auth0Service := services.NewAuth0Service(cfg.Auth0Domain)
	sessionService := services.NewSessionService().WithNotifier(notificationService)
	rsvpService := services.NewRSVPService()
	// Removing a member gives their upcoming spots back, which is the RSVP
	// service's job — it owns the session lock and the waitlist promotion.
	userService := services.NewUserService(cfg.AdminEmail).WithRSVPs(rsvpService)

	// Initialize scheduler for notification cron jobs
	var scheduler *services.SchedulerService
	if !cfg.NotificationsDisabled {
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
	settlementService := services.NewSettlementService(ledgerService).WithNotifier(notificationService)
	settlementHandler := handlers.NewSettlementHandler(settlementService)
	expenseService := services.NewExpenseService(settlementService, ledgerService)
	expenseHandler := handlers.NewExpenseHandler(expenseService)
	var planner assistant.Planner
	var transcriber assistant.Transcriber
	if cfg.GroqAPIKey != "" {
		provider := assistant.NewGroq(cfg.GroqAPIKey, cfg.GroqModel, cfg.GroqSpeechModel)
		planner, transcriber = provider, provider
	}
	assistantHandler := handlers.NewAssistantHandler(services.NewAssistantService(planner, transcriber, expenseService, ledgerService))

	// Auth0 config for middleware
	auth0Config := middleware.Auth0Config{
		Domain:   cfg.Auth0Domain,
		Audience: cfg.Auth0Audience,
	}

	// Setup router
	r := gin.New()
	r.Use(obs.Middleware())
	// Do not dump request headers or bodies into panic logs.
	r.Use(gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered interface{}) {
		log.Printf("HTTP handler panic\n%s", debug.Stack())
		c.AbortWithStatus(http.StatusInternalServerError)
	}))

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
		ledgerHandler.RegisterRoutes(protected)
		adminHandler.RegisterMemberPushRoutes(protected)
		notificationHandler.RegisterAdminRoutes(protected)
		expenseHandler.RegisterRoutes(protected)
		assistantHandler.RegisterRoutes(protected)
		handlers.NewGameHandler(services.NewGameService()).RegisterRoutes(protected)
		{
			// User routes
			invitationHandler.RegisterRoutes(protected)
			protected.GET("/users/me", userHandler.GetMe)
			protected.PUT("/users/me", userHandler.UpdateMe)

			// Notification preferences routes (available to all authenticated users)
			protected.GET("/users/me/notifications", notificationHandler.GetPreferences)
			protected.PUT("/users/me/notifications", notificationHandler.ObsoletePushMutation)
			protected.POST("/users/me/push-tokens", notificationHandler.ObsoletePushMutation)
			protected.DELETE("/users/me/push-tokens", notificationHandler.ObsoletePushMutation)
			notificationHandler.RegisterHistoryRoutes(protected)
			notificationHandler.RegisterDeviceRoutes(protected)

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

				// Session history and settlement breakdowns are readable by any
				// approved member, so the split can be checked by the people in it.
				approved.GET("/sessions/history", settlementHandler.ListSessionHistory)
				approved.GET("/sessions/:id/settlement", settlementHandler.GetSessionSettlement)
			}

			// Admin routes
			admin := approved.Group("/admin")
			admin.Use(middleware.RequireAdmin())
			{
				// Join requests
				admin.GET("/join-requests", adminHandler.ListJoinRequests)
				admin.POST("/join-requests/:id/approve", adminHandler.ApproveJoinRequest)
				admin.POST("/join-requests/:id/reject", adminHandler.RejectJoinRequest)

				// Member management. Removal is a status change, never a
				// delete: RSVPs, ledger entries and settlements reference the
				// row, and the ledger is append-only.
				admin.GET("/users", adminHandler.ListMembers)
				admin.POST("/users", adminHandler.InviteMember)
				admin.PUT("/users/:id", adminHandler.UpdateMember)
				admin.DELETE("/users/:id", adminHandler.RemoveMember)
				admin.POST("/users/:id/reinstate", adminHandler.ReinstateMember)
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

				// Settlement. Preview writes nothing; settle moves money.
				admin.POST("/sessions/:id/settlement/preview", settlementHandler.PreviewSettlement)
				admin.POST("/sessions/:id/settle", settlementHandler.SettleSession)
				admin.POST("/settlements/:id/reverse", settlementHandler.ReverseSettlement)

				// Announcements
				admin.POST("/announcements", notificationHandler.SendAnnouncement)
			}
		}
	}

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r, ReadHeaderTimeout: 10 * time.Second}
	serverError := make(chan error, 1)

	// Start server in goroutine
	go func() {
		log.Printf("Server starting on port %s", cfg.Port)
		serverError <- srv.ListenAndServe()
	}()

	// Wait for shutdown signal
	select {
	case <-quit:
	case err := <-serverError:
		if scheduler != nil {
			scheduler.Stop()
		}
		return fmt.Errorf("server stopped: %w", err)
	}
	log.Println("Shutting down server...")

	// Stop scheduler if running
	if scheduler != nil {
		scheduler.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		_ = srv.Close()
		return err
	}
	log.Println("Server stopped")
	return nil
}
