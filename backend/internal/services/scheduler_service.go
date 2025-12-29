package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
)

type SchedulerService struct {
	cron                *cron.Cron
	notificationService *NotificationService
	reminderHours24     int
	reminderHours12     int
	deadlineHours       int
}

type SchedulerConfig struct {
	NotificationService    *NotificationService
	SessionReminderHours24 int
	SessionReminderHours12 int
	DeadlineReminderHours  int
}

// NewSchedulerService creates a new scheduler service for notification cron jobs
func NewSchedulerService(cfg SchedulerConfig) *SchedulerService {
	return &SchedulerService{
		cron:                cron.New(cron.WithSeconds()),
		notificationService: cfg.NotificationService,
		reminderHours24:     cfg.SessionReminderHours24,
		reminderHours12:     cfg.SessionReminderHours12,
		deadlineHours:       cfg.DeadlineReminderHours,
	}
}

// Start begins the scheduler cron jobs
func (s *SchedulerService) Start() {
	// Run every hour at minute 0 to check for reminders
	// This runs at :00 of each hour
	_, err := s.cron.AddFunc("0 0 * * * *", func() {
		s.checkSessionReminders()
		s.checkDeadlineReminders()
	})
	if err != nil {
		log.Printf("Failed to add cron job: %v", err)
		return
	}

	s.cron.Start()
	log.Printf("Scheduler started - Session reminders at %dh and %dh, Deadline alerts at %dh",
		s.reminderHours24, s.reminderHours12, s.deadlineHours)
}

// Stop gracefully stops the scheduler
func (s *SchedulerService) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Println("Scheduler stopped")
}

// checkSessionReminders checks for sessions that need reminders sent
func (s *SchedulerService) checkSessionReminders() {
	now := utils.NowInSydney()
	log.Printf("Checking session reminders at %s", now.Format("2006-01-02 15:04"))

	// Check for 24h reminders
	s.sendSessionRemindersForWindow(now, s.reminderHours24, "24h")

	// Check for 12h reminders
	s.sendSessionRemindersForWindow(now, s.reminderHours12, "12h")
}

// sendSessionRemindersForWindow sends reminders for sessions starting within a time window
func (s *SchedulerService) sendSessionRemindersForWindow(now time.Time, hoursAhead int, label string) {
	// Calculate the target time window (e.g., 24h from now, within a 1-hour window)
	windowStart := now.Add(time.Duration(hoursAhead) * time.Hour)
	windowEnd := windowStart.Add(1 * time.Hour)

	// Find sessions starting within this window
	var sessions []models.Session
	err := database.DB.Where(
		"session_date = ? AND status = ?",
		windowStart.Format("2006-01-02"),
		models.SessionStatusOpen,
	).Find(&sessions).Error

	if err != nil {
		log.Printf("Error fetching sessions for %s reminders: %v", label, err)
		return
	}

	for _, session := range sessions {
		// Parse session start time and check if it falls within our window
		sessionStart, err := s.parseSessionDateTime(session)
		if err != nil {
			log.Printf("Error parsing session time: %v", err)
			continue
		}

		if sessionStart.After(windowStart) && sessionStart.Before(windowEnd) {
			s.sendSessionReminders(session, label)
		}
	}
}

// sendSessionReminders sends reminders to all users who have RSVP'd to a session
func (s *SchedulerService) sendSessionReminders(session models.Session, label string) {
	ctx := context.Background()

	// Get all RSVPs with status "in" for this session
	var rsvps []models.RSVP
	err := database.DB.Where("session_id = ? AND status = ?", session.ID, models.RSVPStatusIn).Find(&rsvps).Error
	if err != nil {
		log.Printf("Error fetching RSVPs for session %s: %v", session.ID, err)
		return
	}

	if len(rsvps) == 0 {
		return
	}

	// Format session date for display
	dateStr := utils.FormatDateForDisplay(session.SessionDate)

	for _, rsvp := range rsvps {
		title := fmt.Sprintf("Session Reminder (%s)", label)
		body := fmt.Sprintf("Don't forget! %s is on %s at %s", session.Title, dateStr, session.StartTime)
		data := map[string]string{
			"type":       string(models.NotificationSessionReminder),
			"session_id": session.ID.String(),
		}

		if err := s.notificationService.SendNotification(ctx, rsvp.UserID, models.NotificationSessionReminder, title, body, data); err != nil {
			log.Printf("Error sending session reminder to user %s: %v", rsvp.UserID, err)
		}
	}

	log.Printf("Sent %s session reminders to %d users for session %s", label, len(rsvps), session.Title)
}

// checkDeadlineReminders checks for sessions with approaching RSVP deadlines
func (s *SchedulerService) checkDeadlineReminders() {
	now := utils.NowInSydney()
	ctx := context.Background()

	// Calculate the deadline window (e.g., deadlines within the next 6 hours)
	windowStart := now
	windowEnd := now.Add(time.Duration(s.deadlineHours) * time.Hour)

	// Find sessions with deadlines in this window that are still open
	var sessions []models.Session
	err := database.DB.Where(
		"rsvp_deadline > ? AND rsvp_deadline <= ? AND status = ?",
		windowStart,
		windowEnd,
		models.SessionStatusOpen,
	).Find(&sessions).Error

	if err != nil {
		log.Printf("Error fetching sessions for deadline reminders: %v", err)
		return
	}

	for _, session := range sessions {
		s.sendDeadlineReminders(ctx, session)
	}
}

// sendDeadlineReminders sends deadline alerts to users who haven't RSVP'd yet
func (s *SchedulerService) sendDeadlineReminders(ctx context.Context, session models.Session) {
	// Get all approved members
	var users []models.User
	err := database.DB.Where("membership_status = ?", models.MembershipApproved).Find(&users).Error
	if err != nil {
		log.Printf("Error fetching users for deadline reminders: %v", err)
		return
	}

	// Get existing RSVPs for this session
	var existingRSVPs []models.RSVP
	database.DB.Where("session_id = ?", session.ID).Find(&existingRSVPs)

	// Build map of users who have already RSVP'd
	rsvpUserMap := make(map[uuid.UUID]bool)
	for _, rsvp := range existingRSVPs {
		rsvpUserMap[rsvp.UserID] = true
	}

	// Format deadline for display
	deadlineStr := session.RSVPDeadline.In(utils.SydneyLocation).Format("Monday 3:04 PM")
	dateStr := utils.FormatDateForDisplay(session.SessionDate)

	notifiedCount := 0
	for _, user := range users {
		// Skip users who have already RSVP'd
		if rsvpUserMap[user.ID] {
			continue
		}

		title := "RSVP Deadline Approaching"
		body := fmt.Sprintf("The RSVP deadline for %s (%s) is %s. Don't miss out!", session.Title, dateStr, deadlineStr)
		data := map[string]string{
			"type":       string(models.NotificationRSVPDeadline),
			"session_id": session.ID.String(),
		}

		if err := s.notificationService.SendNotification(ctx, user.ID, models.NotificationRSVPDeadline, title, body, data); err != nil {
			log.Printf("Error sending deadline reminder to user %s: %v", user.ID, err)
		} else {
			notifiedCount++
		}
	}

	if notifiedCount > 0 {
		log.Printf("Sent RSVP deadline reminders to %d users for session %s", notifiedCount, session.Title)
	}
}

// parseSessionDateTime parses a session's date and start time into a time.Time
func (s *SchedulerService) parseSessionDateTime(session models.Session) (time.Time, error) {
	// session.SessionDate is already a time.Time (date only)
	// session.StartTime is a string like "18:30"

	dateInSydney := session.SessionDate.In(utils.SydneyLocation)

	// Parse start time
	startTime, err := time.Parse("15:04", session.StartTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse start time %s: %w", session.StartTime, err)
	}

	// Combine date and time
	result := time.Date(
		dateInSydney.Year(),
		dateInSydney.Month(),
		dateInSydney.Day(),
		startTime.Hour(),
		startTime.Minute(),
		0, 0,
		utils.SydneyLocation,
	)

	return result, nil
}

// SendWaitlistUpdate sends a notification when a spot opens up
// This should be called from RSVPService when someone cancels their RSVP
func (s *SchedulerService) SendWaitlistUpdate(ctx context.Context, session models.Session) {
	// Get confirmed count
	var confirmedCount int64
	database.DB.Model(&models.RSVP{}).
		Where("session_id = ? AND status = ?", session.ID, models.RSVPStatusIn).
		Count(&confirmedCount)

	// If session is full, no need to notify
	if int(confirmedCount) >= session.MaxPlayers {
		return
	}

	spotsAvailable := session.MaxPlayers - int(confirmedCount)

	// Get users who marked "maybe" or are on the waitlist, ordered by RSVP time
	var maybeRSVPs []models.RSVP
	err := database.DB.Where("session_id = ? AND status = ?", session.ID, models.RSVPStatusMaybe).
		Order("rsvp_timestamp ASC").
		Limit(spotsAvailable).
		Find(&maybeRSVPs).Error

	if err != nil {
		log.Printf("Error fetching maybe RSVPs: %v", err)
		return
	}

	dateStr := utils.FormatDateForDisplay(session.SessionDate)

	for _, rsvp := range maybeRSVPs {
		title := "Spot Available!"
		body := fmt.Sprintf("A spot has opened up for %s on %s. RSVP now to confirm your place!", session.Title, dateStr)
		data := map[string]string{
			"type":       string(models.NotificationWaitlistUpdate),
			"session_id": session.ID.String(),
		}

		if err := s.notificationService.SendNotification(ctx, rsvp.UserID, models.NotificationWaitlistUpdate, title, body, data); err != nil {
			log.Printf("Error sending waitlist update to user %s: %v", rsvp.UserID, err)
		}
	}

	if len(maybeRSVPs) > 0 {
		log.Printf("Sent waitlist updates to %d users for session %s", len(maybeRSVPs), session.Title)
	}
}
