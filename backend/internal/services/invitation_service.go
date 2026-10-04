package services

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:embed invitation_email.html
var invitationHTML string

var invitationTemplate = template.Must(template.New("invitation").Parse(invitationHTML))

type InvitationError struct {
	Status  int
	Message string
}

func (e *InvitationError) Error() string               { return e.Message }
func invitationError(status int, message string) error { return &InvitationError{status, message} }

type InvitationEmail struct {
	Subject        string `json:"subject"`
	HTML           string `json:"html"`
	Text           string `json:"text"`
	LoginURL       string `json:"login_url"`
	RecipientEmail string `json:"recipient_email"`
}

type InvitationPreview struct {
	InvitationEmail
	FromEmail         string                     `json:"from_email"`
	TestRecipient     string                     `json:"test_recipient"`
	CanSend           bool                       `json:"can_send"`
	CanTest           bool                       `json:"can_test"`
	SendBlockedReason string                     `json:"send_blocked_reason,omitempty"`
	TestBlockedReason string                     `json:"test_blocked_reason,omitempty"`
	LastInvitation    *models.InvitationDelivery `json:"last_invitation,omitempty"`
	LastTest          *models.InvitationDelivery `json:"last_test,omitempty"`
}

type InvitationService struct {
	cfg         NotificationConfig
	testEnabled bool
	send        func(context.Context, string, InvitationEmail) (string, string)
	newClient   func() *sendgrid.Client
}

func NewInvitationService(cfg NotificationConfig, testEnabled bool) *InvitationService {
	s := &InvitationService{cfg: cfg, testEnabled: testEnabled}
	s.newClient = func() *sendgrid.Client { return sendgrid.NewSendClient(cfg.SendGridAPIKey) }
	s.send = s.sendEmail
	return s
}

func (s *InvitationService) members(actorID, userID uuid.UUID) (*models.User, *models.User, error) {
	var actor, user models.User
	if err := database.DB.First(&actor, "id = ?", actorID).Error; err != nil || !actor.IsApproved() || !actor.IsAdmin() || !actor.HasSignedIn() {
		return nil, nil, invitationError(403, "Only a signed-in club admin can send invitations.")
	}
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, invitationError(404, "Member not found.")
		}
		return nil, nil, err
	}
	if !user.IsApproved() || user.HasSignedIn() {
		return nil, nil, invitationError(409, "Invitations are only for approved members who have not signed in.")
	}
	return &actor, &user, nil
}

func (s *InvitationService) render(user models.User, club models.Club) (InvitationEmail, error) {
	u, err := url.Parse(s.cfg.FrontendURL)
	if err != nil || u == nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) {
		return InvitationEmail{}, invitationError(503, "The invitation sign-in address is not configured correctly.")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/welcome"
	data := struct{ Name, Email, ClubName, Subject, LoginURL string }{user.DisplayName(), user.Email, club.Name, "You're invited to " + club.Name + " on Rally", u.String()}
	var html bytes.Buffer
	if err := invitationTemplate.Execute(&html, data); err != nil {
		return InvitationEmail{}, err
	}
	text := fmt.Sprintf("Hi %s,\n\nYour place in %s is ready.\n\nSign in with your Google account: %s\n\nOpen Rally: %s\n\nYour existing balance, recorded history, and saved RSVPs stay with this account. If you are already marked IN for a session, you do not need to RSVP again. No new password is needed.\n\nIf a different account opens, sign out of Rally and choose the correct Google account. For help, contact your club admin.\n", data.Name, data.ClubName, data.Email, data.LoginURL)
	return InvitationEmail{data.Subject, html.String(), text, data.LoginURL, user.Email}, nil
}

func (s *InvitationService) blocked(club models.Club, test bool) string {
	if test {
		if !s.testEnabled {
			return "Test email sending is disabled. You can still preview the email."
		}
	} else if s.cfg.Disabled || club.NotificationsPaused {
		return "Member invitations are paused. No member email will be sent."
	}
	if s.cfg.SendGridAPIKey == "" || validateEmail(s.cfg.SendGridFromEmail) != nil {
		return "Email sending is not configured. You can still preview the email."
	}
	return ""
}

func (s *InvitationService) Preview(actorID, userID uuid.UUID) (*InvitationPreview, error) {
	actor, user, err := s.members(actorID, userID)
	if err != nil {
		return nil, err
	}
	var club models.Club
	if err := database.DB.First(&club).Error; err != nil {
		return nil, err
	}
	email, err := s.render(*user, club)
	if err != nil {
		return nil, err
	}
	p := &InvitationPreview{InvitationEmail: email, FromEmail: s.cfg.SendGridFromEmail, TestRecipient: actor.Email, SendBlockedReason: s.blocked(club, false), TestBlockedReason: s.blocked(club, true)}
	p.CanSend, p.CanTest = p.SendBlockedReason == "", p.TestBlockedReason == ""
	for _, test := range []bool{false, true} {
		to := user.Email
		if test {
			to = actor.Email
		}
		var delivery models.InvitationDelivery
		q := database.DB.Where("user_id = ? AND is_test = ? AND recipient_email = ?", userID, test, to)
		if test {
			q = q.Where("actor_id = ?", actorID)
		}
		if err := q.Order("created_at DESC, id DESC").First(&delivery).Error; err == nil {
			if test {
				p.LastTest = &delivery
			} else {
				p.LastInvitation = &delivery
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	return p, nil
}

// Send reserves an attempt before contacting the provider. A lost response can
// be retrieved using the same request ID, without sending another email.
func (s *InvitationService) Send(ctx context.Context, actorID, userID, requestID uuid.UUID, test bool, expectedEmail string) (*models.InvitationDelivery, error) {
	_, user, err := s.members(actorID, userID)
	if err != nil {
		return nil, err
	}
	if requestID == uuid.Nil {
		return nil, invitationError(400, "A request ID is required.")
	}
	var email InvitationEmail
	var delivery models.InvitationDelivery
	fresh := false
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		// Serialize per recipient. For tests, the recipient is always the admin,
		// so opening multiple member previews cannot cause a burst of test mail.
		lockID := userID
		if test {
			lockID = actorID
		}
		var lock models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lock, "id = ?", lockID).Error; err != nil {
			return err
		}
		if test && (!lock.IsApproved() || !lock.IsAdmin() || !lock.HasSignedIn()) {
			return invitationError(403, "Only a signed-in club admin can receive a test invitation.")
		}
		if err := tx.First(user, "id = ?", userID).Error; err != nil {
			return err
		}
		if !user.IsApproved() || user.HasSignedIn() {
			return invitationError(409, "This member can no longer receive an invitation.")
		}
		if user.Email != expectedEmail {
			return invitationError(409, "The member's email changed. Reopen the preview before sending.")
		}
		to := user.Email
		if test {
			to = lock.Email
		}
		if err := tx.Where("request_id = ?", requestID).First(&delivery).Error; err == nil {
			if delivery.UserID != userID || delivery.ActorID != actorID || delivery.IsTest != test || delivery.RecipientEmail != to {
				return invitationError(409, "This request ID was already used for a different email.")
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var club models.Club
		if err := tx.First(&club).Error; err != nil {
			return err
		}
		if reason := s.blocked(club, test); reason != "" {
			return invitationError(409, reason)
		}
		if validateEmail(to) != nil {
			return invitationError(409, "The recipient email is invalid.")
		}
		var recent int64
		if err := tx.Model(&models.InvitationDelivery{}).Where("recipient_email = ? AND created_at > ?", to, time.Now().Add(-time.Minute)).Count(&recent).Error; err != nil {
			return err
		}
		if recent > 0 {
			return invitationError(429, "Wait one minute before sending another email to this address.")
		}
		var err error
		email, err = s.render(*user, club)
		if err != nil {
			return err
		}
		delivery = models.InvitationDelivery{RequestID: requestID, UserID: userID, ActorID: actorID, RecipientEmail: to, IsTest: test, Status: "pending", Message: "Submission started. Delivery is not yet confirmed."}
		if err := tx.Create(&delivery).Error; err != nil {
			return err
		}
		fresh = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !fresh {
		return &delivery, nil
	}
	if test {
		email.Subject = "[TEST] " + email.Subject
	}
	// Continue for a bounded time if the browser disconnects, to persist the
	// provider result instead of silently starting another send on retry.
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	delivery.Status, delivery.Message = s.send(sendCtx, delivery.RecipientEmail, email)
	if delivery.Status == "accepted" {
		now := time.Now()
		delivery.AcceptedAt = &now
	}
	if err := database.DB.Model(&delivery).Updates(map[string]any{"status": delivery.Status, "message": delivery.Message, "accepted_at": delivery.AcceptedAt}).Error; err != nil {
		return nil, invitationError(503, "Email submission may have completed, but its result could not be saved. Check your inbox before trying again.")
	}
	return &delivery, nil
}

func (s *InvitationService) sendEmail(ctx context.Context, to string, email InvitationEmail) (string, string) {
	message := mail.NewSingleEmail(mail.NewEmail(s.cfg.SendGridFromName, s.cfg.SendGridFromEmail), email.Subject, mail.NewEmail("", to), email.Text, email.HTML)
	off := false
	message.SetTrackingSettings(&mail.TrackingSettings{ClickTracking: &mail.ClickTrackingSetting{Enable: &off, EnableText: &off}, OpenTracking: &mail.OpenTrackingSetting{Enable: &off}})
	response, err := s.newClient().SendWithContext(ctx, message)
	if err != nil {
		return "unknown", "The email provider did not confirm the result. Check the inbox before resending."
	}
	if response.StatusCode == 202 {
		return "accepted", "Accepted by the email provider. Inbox delivery is not yet confirmed."
	}
	if response.StatusCode >= 400 && response.StatusCode < 500 {
		// Return actionable categories, never raw provider bodies or credentials.
		body := strings.ToLower(response.Body)
		if strings.Contains(body, "maximum credits exceeded") {
			return "failed", "SendGrid's sending limit has been reached. Restore the account's sending allowance before trying again. No email was accepted."
		}
		if strings.Contains(body, "verified sender") || strings.Contains(body, "sender identity") {
			return "failed", "SendGrid did not accept the sender address. Verify the sender in SendGrid before trying again."
		}
		return "failed", fmt.Sprintf("The email provider rejected the message (HTTP %d). Check the sender, API key, and sending allowance before trying again.", response.StatusCode)
	}
	return "unknown", "The email provider returned an unexpected response. Check the inbox before resending."
}
