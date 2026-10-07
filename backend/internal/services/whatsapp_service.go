package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const WhatsAppFallbackDelay = 15 * time.Minute
const whatsAppMonthlyBudgetCents int64 = 500
const whatsAppMonthlyMessageLimit = 200

var ErrWhatsAppPhone = errors.New("save a valid Australian mobile number before enabling WhatsApp alerts")
var ErrPushReceipt = errors.New("invalid or expired push receipt")
var auMobilePattern = regexp.MustCompile(`^614[0-9]{8}$`)
var graphVersionPattern = regexp.MustCompile(`^v[0-9]+\.0$`)
var numericIDPattern = regexp.MustCompile(`^[0-9]+$`)

// Only Australian mobile numbers are enabled for this release and its budget.
func australianMobile(raw string) (string, error) {
	phone := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(raw))
	phone = strings.TrimPrefix(phone, "+")
	if strings.HasPrefix(phone, "04") {
		phone = "61" + phone[1:]
	}
	if !auMobilePattern.MatchString(phone) {
		return "", ErrWhatsAppPhone
	}
	return phone, nil
}

type WhatsAppConfig struct {
	Enabled          bool
	AccessToken      string
	PhoneNumberID    string
	GraphVersion     string
	LowTemplate      string
	NegativeTemplate string
	Language         string
	// Whole cents, rounded UP from the tax-inclusive AUD price, at least 2.
	ReservedCents int64
}

type WhatsAppSender struct {
	config   WhatsAppConfig
	client   *http.Client
	endpoint string
}

func NewWhatsAppSender(cfg WhatsAppConfig) *WhatsAppSender {
	if !cfg.Enabled || cfg.AccessToken == "" || !numericIDPattern.MatchString(cfg.PhoneNumberID) ||
		!graphVersionPattern.MatchString(cfg.GraphVersion) || cfg.LowTemplate == "" || cfg.NegativeTemplate == "" || cfg.Language == "" ||
		cfg.ReservedCents < 2 || cfg.ReservedCents > whatsAppMonthlyBudgetCents {
		return nil
	}
	return &WhatsAppSender{config: cfg, client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
		endpoint: "https://graph.facebook.com/" + cfg.GraphVersion + "/" + cfg.PhoneNumberID + "/messages"}
}

// send returns provider acceptance, not delivery. Never retry an ambiguous POST.
func (w *WhatsAppSender) send(ctx context.Context, phone string, kind models.NotificationType, balance int64) (string, error) {
	template := w.config.LowTemplate
	if kind == models.NotificationBalanceNegative {
		template = w.config.NegativeTemplate
	}
	body, err := json.Marshal(map[string]interface{}{
		"messaging_product": "whatsapp", "recipient_type": "individual", "to": phone, "type": "template",
		"template": map[string]interface{}{"name": template, "language": map[string]string{"code": w.config.Language},
			"components": []interface{}{map[string]interface{}{"type": "body", "parameters": []interface{}{map[string]string{"type": "text", "text": formatCents(balance)}}}}},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+w.config.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	response, err := w.client.Do(req)
	if err != nil {
		return "", errors.New("WhatsApp request outcome is unknown")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("WhatsApp HTTP status %d", response.StatusCode)
	}
	var result struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&result); err != nil || len(result.Messages) != 1 || result.Messages[0].ID == "" {
		return "", errors.New("WhatsApp acceptance could not be confirmed")
	}
	return result.Messages[0].ID, nil
}

func isBalanceAlert(kind models.NotificationType) bool {
	return kind == models.NotificationBalanceLow || kind == models.NotificationBalanceNegative
}

func receiptHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *NotificationService) prepareWhatsAppFallback(n *models.Notification, user *models.User, prefs *models.UserNotificationPreferences, data map[string]string) error {
	// Explicit admin nudges retain their existing push-only behavior.
	if s.whatsapp == nil || !isBalanceAlert(n.NotificationType) || data["source"] == "admin_balance_nudge" || n.WhatsAppStatus != "" || !prefs.WhatsAppBalanceAlerts {
		return nil
	}
	phone, err := australianMobile(user.PhoneNumber)
	if err != nil || phone != prefs.WhatsAppConsentPhone {
		return nil
	}
	var devices int64
	if s.fcmEnabled && prefs.IsPushEnabledForType(n.NotificationType) {
		if err := database.DB.Model(&models.UserPushToken{}).Where("user_id = ?", user.ID).Count(&devices).Error; err != nil {
			return err
		}
	}
	due := time.Now()
	if devices > 0 {
		due = due.Add(WhatsAppFallbackDelay)
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	token := hex.EncodeToString(secret)
	n.PushReceiptHash = receiptHash(token)
	n.WhatsAppStatus = "pending"
	n.WhatsAppPhone = phone
	n.WhatsAppDueAt = &due

	data["receipt_token"] = token
	return nil
}

// A receipt only authorizes marking this one notification as received. It cannot
// read balances, alter preferences or act as a login. Hashes never leave the API.
func (s *NotificationService) RecordPushReceipt(id uuid.UUID, token string) error {
	if len(token) != 64 {
		return ErrPushReceipt
	}
	now := time.Now()
	result := database.DB.Model(&models.Notification{}).
		Where("id = ? AND push_receipt_hash = ? AND created_at > ?", id, receiptHash(token), now.Add(-24*time.Hour)).
		Updates(map[string]interface{}{"push_received_at": gorm.Expr("COALESCE(push_received_at, ?)", now),
			"whats_app_status": gorm.Expr("CASE WHEN whats_app_status = 'pending' THEN 'cancelled' ELSE whats_app_status END")})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrPushReceipt
	}
	return nil
}

// ProcessWhatsAppFallbacks can run on every replica. A database advisory lock
// serializes reservation of the shared monthly budget; row claims persist before
// network I/O. A crash after claiming deliberately leaves an uncertain send and
// its budget reservation, rather than risking duplicate paid requests.
func (s *NotificationService) ProcessWhatsAppFallbacks(ctx context.Context) error {
	if s.disabled || s.whatsapp == nil {
		return nil
	}
	var pending []models.Notification
	now := time.Now()
	if err := database.DB.WithContext(ctx).Where("whats_app_status = ? AND whats_app_due_at <= ?", "pending", now).
		Order("whats_app_due_at, id").Limit(100).Find(&pending).Error; err != nil {
		return err
	}
	for _, n := range pending {
		if err := s.dispatchWhatsApp(ctx, n.ID, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *NotificationService) dispatchWhatsApp(ctx context.Context, id uuid.UUID, now time.Time) error {
	var n models.Notification
	var balance int64
	claimed := false
	err := database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Stable application-specific key, shared by all replicas and all months.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(823740501)").Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&n, "id = ?", id).Error; err != nil {
			return err
		}
		if n.WhatsAppStatus != "pending" || n.WhatsAppDueAt == nil || n.WhatsAppDueAt.After(now) {
			return nil
		}
		cancel := func(status string) error { return tx.Model(&n).Update("whats_app_status", status).Error }
		var club models.Club
		if err := tx.First(&club).Error; err != nil {
			return err
		}
		if club.NotificationsPaused {
			return nil
		}
		if n.PushReceivedAt != nil || n.ReadAt != nil || n.CreatedAt.Before(now.Add(-24*time.Hour)) || !isBalanceAlert(n.NotificationType) {
			return cancel("cancelled")
		}
		var user models.User
		if err := tx.First(&user, "id = ?", n.UserID).Error; err != nil {
			return err
		}
		var prefs models.UserNotificationPreferences
		if err := tx.Where("user_id = ?", n.UserID).First(&prefs).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return cancel("cancelled")
			}
			return err
		}
		phone, err := australianMobile(user.PhoneNumber)
		if err != nil || user.MembershipStatus != models.MembershipApproved || !prefs.WhatsAppBalanceAlerts ||
			phone != n.WhatsAppPhone || phone != prefs.WhatsAppConsentPhone {
			return cancel("cancelled")
		}
		if err := tx.Raw(`SELECT COALESCE(SUM(e.amount_cents), 0) FROM accounts a
   LEFT JOIN ledger_entries e ON e.account_id = a.id WHERE a.user_id = ?`, n.UserID).Scan(&balance).Error; err != nil {
			return err
		}
		if (n.NotificationType == models.NotificationBalanceNegative && balance >= 0) ||
			(n.NotificationType == models.NotificationBalanceLow && (balance >= club.LowBalanceThresholdCents || balance < 0)) {
			return cancel("cancelled")
		}
		// Don't send an old alert after a recovery followed by another drop. A positive
		// ledger posting that reached this boundary ends this alert's episode.
		var recovered bool
		boundary := club.LowBalanceThresholdCents
		if n.NotificationType == models.NotificationBalanceNegative {
			boundary = 0
		}
		if err := tx.Raw(`SELECT EXISTS (
   SELECT 1 FROM (SELECT e.amount_cents, e.created_at,
    SUM(e.amount_cents) OVER (ORDER BY e.created_at, e.id) AS balance
    FROM ledger_entries e JOIN accounts a ON a.id = e.account_id WHERE a.user_id = ?) h
   WHERE h.created_at > ? AND h.amount_cents > 0 AND h.balance >= ?
  )`, n.UserID, n.CreatedAt, boundary).Scan(&recovered).Error; err != nil {
			return err
		}
		if recovered {
			return cancel("cancelled")
		}
		local := now.In(utils.SydneyLocation)
		month := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, utils.SydneyLocation)
		var usage struct {
			Cents int64
			Count int64
		}
		if err := tx.Model(&models.Notification{}).Select("COALESCE(SUM(whats_app_reserved_cents), 0) AS cents, COUNT(*) AS count").
			Where("whats_app_reserved_at >= ? AND whats_app_reserved_at < ?", month, month.AddDate(0, 1, 0)).Scan(&usage).Error; err != nil {
			return err
		}
		if usage.Cents+s.whatsapp.config.ReservedCents > whatsAppMonthlyBudgetCents || usage.Count >= whatsAppMonthlyMessageLimit {
			return cancel("budget_blocked")
		}
		if err := tx.Model(&n).Updates(map[string]interface{}{"whats_app_status": "claimed", "whats_app_reserved_at": now,
			"whats_app_reserved_cents": s.whatsapp.config.ReservedCents}).Error; err != nil {
			return err
		}
		claimed = true
		return nil
	})
	if err != nil || !claimed {
		return err
	}
	messageID, sendErr := s.whatsapp.send(ctx, n.WhatsAppPhone, n.NotificationType, balance)
	status := "accepted"
	if sendErr != nil {
		status = "uncertain"
		log.Printf("WhatsApp notification %s has an uncertain outcome: %v", n.ID, sendErr)
	}
	// Do not log provider responses, tokens, phone numbers or balances.
	return database.DB.WithContext(ctx).Model(&n).Updates(map[string]interface{}{
		"whats_app_status": status, "whats_app_message_id": messageID,
	}).Error
}
