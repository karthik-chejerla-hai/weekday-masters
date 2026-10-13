package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sort"
	"strconv"
	"time"
)

func BalanceNotificationBand(balance int64) string {
	if balance < 0 {
		return "negative"
	}
	if balance < models.NotificationBalanceThresholdCents {
		return "low"
	}
	return "healthy"
}
func BalanceWarning(balance int64) string {
	switch BalanceNotificationBand(balance) {
	case "negative":
		return "Your balance is negative. Please top up."
	case "low":
		return "Your balance is below $20. Please top up."
	default:
		return ""
	}
}
func BalanceAlertText(balance int64) string {
	message := "Your balance is " + formatCents(balance) + "."
	if warning := BalanceWarning(balance); warning != "" {
		message += " " + warning
	}
	return message
}
func balanceWithin(tx *gorm.DB, userID uuid.UUID) (int64, error) {
	var balance int64
	err := tx.Raw(`SELECT COALESCE(SUM(e.amount_cents),0) FROM accounts a LEFT JOIN ledger_entries e ON e.account_id=a.id WHERE a.user_id=?`, userID).Scan(&balance).Error
	return balance, err
}

func (s *SettlementService) queueChargesWithin(tx *gorm.DB, session models.Session, record models.Settlement, lines []ChargeLineView, reversal *models.Transaction) error {
	charged := map[uuid.UUID]int64{}
	for _, line := range lines {
		charged[line.UserID] += line.AmountCents
	}
	ids := make([]uuid.UUID, 0, len(charged))
	for id := range charged {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	for _, userID := range ids {
		balance, err := balanceWithin(tx, userID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		kind := models.NotificationSessionCharge
		source := "session_charge"
		key := fmt.Sprintf("charge:%s:%s", record.ID, userID)
		title := "Session charges"
		body := fmt.Sprintf("Charged %s for %s. Balance after this split: %s.", formatCents(charged[userID]), session.Title, formatCents(balance))
		data := map[string]string{"type": string(kind), "source": source, "session_id": session.ID.String(), "settlement_id": record.ID.String(), "charge_cents": strconv.FormatInt(charged[userID], 10), "balance_after_cents": strconv.FormatInt(balance, 10), "balance_band": BalanceNotificationBand(balance), "checked_at": now.Format(time.RFC3339Nano)}
		if reversal != nil {
			kind = models.NotificationSessionChargeReversed
			title = "Session charge reversed"
			key = fmt.Sprintf("charge-reversal:%s:%s:%s", record.ID, reversal.ID, userID)
			body = fmt.Sprintf("The %s charge for %s was reversed. Balance after this correction: %s.", formatCents(charged[userID]), session.Title, formatCents(balance))
			data["type"] = string(kind)
			data["source"] = "session_charge_reversal"
			data["reversal_transaction_id"] = reversal.ID.String()
		}
		if warning := BalanceWarning(balance); warning != "" {
			body += " " + warning
		}
		notification := models.Notification{UserID: userID, NotificationType: kind, Title: title, Body: body, Data: mustNotificationJSON(data)}
		queue := s.notifier
		if queue == nil {
			queue = &NotificationService{disabled: true}
		}
		if _, err := queue.QueueNotificationWithin(tx, &notification, key, now, now.Add(24*time.Hour)); err != nil {
			return err
		}
	}
	return nil
}
func mustNotificationJSON(data map[string]string) string {
	encoded, _ := json.Marshal(data)
	return string(encoded)
}

// Pending original receipts are suppressed under the same transaction as the
// reversing ledger entry. A sending attempt is uncertain: it cannot be recalled.
func suppressSettlementReceiptsWithin(tx *gorm.DB, settlementID uuid.UUID) error {
	var outboxes []models.NotificationOutbox
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_key LIKE ? AND state IN ?", "charge:"+settlementID.String()+":%", []models.DeliveryState{models.DeliveryPending, models.DeliveryClaimed}).Order("id").Find(&outboxes).Error; err != nil {
		return err
	}
	for _, outbox := range outboxes {
		if err := tx.Model(&models.NotificationDeliveryAttempt{}).Where("outbox_id = ? AND state IN ?", outbox.ID, []models.DeliveryState{models.DeliveryPending, models.DeliveryTransient}).Updates(map[string]any{"state": models.DeliverySuppressed, "last_error_code": "split_reversed"}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.NotificationDeliveryAttempt{}).Where("outbox_id = ? AND state = ?", outbox.ID, models.DeliverySending).Updates(map[string]any{"state": models.DeliveryUncertain, "last_error_code": "split_reversed_inflight"}).Error; err != nil {
			return err
		}
		var attempts []models.NotificationDeliveryAttempt
		if err := tx.Where("outbox_id = ?", outbox.ID).Find(&attempts).Error; err != nil {
			return err
		}
		accepted, uncertain, other := 0, false, false
		for _, attempt := range attempts {
			if attempt.State == models.DeliveryAccepted {
				accepted++
			} else {
				other = true
			}
			if attempt.State == models.DeliveryUncertain {
				uncertain = true
			}
		}
		state := models.DeliverySuppressed
		if accepted > 0 {
			state = models.DeliveryCompleted
			if other {
				state = models.DeliveryPartial
			}
		} else if uncertain {
			state = models.DeliveryUncertain
		}
		if err := tx.Model(&models.NotificationOutbox{}).Where("id = ?", outbox.ID).Updates(map[string]any{"state": state, "last_error_code": "split_reversed", "lease_until": nil, "lease_owner": uuid.New(), "lease_generation": gorm.Expr("lease_generation + 1")}).Error; err != nil {
			return err
		}
	}
	return nil
}
func ManualBalanceMessage(balance int64, checkedAt time.Time) string {
	return BalanceAlertText(balance) + " Balance checked " + checkedAt.In(utils.SydneyLocation).Format("2 Jan 2006 at 3:04 PM") + " Sydney time."
}

var ErrNudgeRequestRequired = errors.New("a valid request_id is required for a balance nudge")
var ErrNudgeRequestConflict = errors.New("request_id was already used for another member")
