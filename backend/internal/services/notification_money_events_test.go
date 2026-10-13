package services

import (
	"context"
	"encoding/json"
	"errors"
	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBalanceWarningBoundaries(t *testing.T) {
	for _, c := range []struct {
		balance int64
		band    string
		warning bool
	}{{-1, "negative", true}, {0, "low", true}, {1999, "low", true}, {2000, "healthy", false}} {
		if BalanceNotificationBand(c.balance) != c.band || (BalanceWarning(c.balance) != "") != c.warning {
			t.Fatalf("incorrect balance classification at %d", c.balance)
		}
	}
}
func receiptFor(t *testing.T, settlementID, userID uuid.UUID, kind models.NotificationType) models.Notification {
	t.Helper()
	var n models.Notification
	if err := database.DB.Where("user_id = ? AND notification_type = ? AND data->>'settlement_id' = ?", userID, kind, settlementID.String()).First(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}
func moneyFacts(t *testing.T, n models.Notification) map[string]string {
	t.Helper()
	var data map[string]string
	if err := json.Unmarshal([]byte(n.Data), &data); err != nil {
		t.Fatal(err)
	}
	return data
}
func TestSessionChargeReceiptAggregatesHostedGuestBandsAndZeroCost(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	host := f.member(t, "host")
	comped := f.member(t, "comped")
	extra := 1.0
	record, preview, err := f.settlement.Settle(SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID, ExtraHours: &extra, Lines: []LineInput{{UserID: host.ID, InBase: true, InExtra: true}, {UserID: host.ID, GuestName: "Guest", InBase: true}, {UserID: comped.ID, InBase: true, InExtra: true, Comped: true}}})
	if err != nil {
		t.Fatal(err)
	}
	var amount int64
	for _, line := range preview.Lines {
		if line.UserID == host.ID {
			amount += line.AmountCents
		}
	}
	n := receiptFor(t, record.ID, host.ID, models.NotificationSessionCharge)
	facts := moneyFacts(t, n)
	balance, err := f.ledger.BalanceOfUser(host.ID)
	if err != nil {
		t.Fatal(err)
	}
	if facts["charge_cents"] != strconv.FormatInt(amount, 10) || facts["balance_after_cents"] != strconv.FormatInt(balance, 10) || facts["balance_band"] != "negative" {
		t.Fatalf("receipt does not match posted amounts: %v", facts)
	}
	zero := moneyFacts(t, receiptFor(t, record.ID, comped.ID, models.NotificationSessionCharge))
	if zero["charge_cents"] != "0" || zero["balance_after_cents"] != "0" || zero["balance_band"] != "low" {
		t.Fatalf("zero-cost participant omitted or misclassified: %v", zero)
	}
	var count int64
	database.DB.Model(&models.Notification{}).Where("data->>'settlement_id' = ?", record.ID.String()).Count(&count)
	if count != 2 {
		t.Fatalf("expected one receipt per liable member, got %d", count)
	}
	if _, err := f.ledger.RecordTopup(CashInput{UserID: host.ID, AmountCents: 20000, CreatedBy: f.admin.ID}); err != nil {
		t.Fatal(err)
	}
	if got := moneyFacts(t, receiptFor(t, record.ID, host.ID, models.NotificationSessionCharge)); got["balance_after_cents"] != facts["balance_after_cents"] {
		t.Fatal("delayed receipt snapshot changed")
	}
	assertBalanced(t)
}
func TestSessionReceiptFailureRollsBackSettlementAndLedger(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")
	stockBefore, err := f.ledger.StockPosition(database.DB)
	if err != nil {
		t.Fatal(err)
	}
	const callback = "test:fail_session_receipt"
	if err := database.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "notifications" {
			tx.AddError(errors.New("intent unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer database.DB.Callback().Create().Remove(callback)
	if _, _, err := f.settlement.Settle(SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID, Lines: []LineInput{{UserID: player.ID, InBase: true}}}); err == nil {
		t.Fatal("settlement committed without receipt intent")
	}
	var count int64
	database.DB.Model(&models.Settlement{}).Count(&count)
	if count != 0 {
		t.Fatal("partial settlement committed")
	}
	database.DB.Model(&models.Transaction{}).Where("kind = ?", models.TxnSessionSettlement).Count(&count)
	if count != 0 {
		t.Fatal("charges committed without intent")
	}
	after, err := f.ledger.StockPosition(database.DB)
	if err != nil || after != stockBefore {
		t.Fatalf("rollback consumed stock: %+v %v", after, err)
	}
	assertBalanced(t)
}
func TestReversalCorrectionAtomicAndOriginalSplitHistorical(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")
	ns := &NotificationService{fcmEnabled: true, fcmClient: &ttlPushClient{}}
	addDeliveryDevice(t, player, "receipt-device")
	f.settlement.WithNotifier(ns)
	input := SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID, Lines: []LineInput{{UserID: player.ID, InBase: true}}}
	record, _, err := f.settlement.Settle(input)
	if err != nil {
		t.Fatal(err)
	}
	original := receiptFor(t, record.ID, player.ID, models.NotificationSessionCharge)
	snapshot := original.Data
	if _, err := f.ledger.ReverseTransaction(record.TransactionID, "bypass", f.admin.ID); err == nil {
		t.Fatal("generic reversal bypassed settlement state")
	}
	reversal, err := f.settlement.ReverseSettlement(record.ID, "Correction", f.admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	correction := moneyFacts(t, receiptFor(t, record.ID, player.ID, models.NotificationSessionChargeReversed))
	if correction["reversal_transaction_id"] != reversal.ID.String() || correction["balance_after_cents"] != "0" {
		t.Fatalf("incorrect correction snapshot: %v", correction)
	}
	var pending models.NotificationOutbox
	if err := database.DB.Where("notification_id = ?", original.ID).First(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if pending.State != models.DeliverySuppressed || pending.LastErrorCode != "split_reversed" {
		t.Fatal("pending original receipt not suppressed")
	}
	if receiptFor(t, record.ID, player.ID, models.NotificationSessionCharge).Data != snapshot {
		t.Fatal("original receipt mutated")
	}
	second, _, err := f.settlement.Settle(input)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == record.ID {
		t.Fatal("re-settlement reused identity")
	}
	oldView, err := f.settlement.SettlementForSessionVersion(f.session.ID, record.ID)
	if err != nil || oldView == nil || oldView.ReversedAt == nil || *oldView.SettlementID != record.ID {
		t.Fatalf("original receipt points at replacement: %+v %v", oldView, err)
	}
	live, err := f.settlement.SettlementForSession(f.session.ID)
	if err != nil || *live.SettlementID != second.ID {
		t.Fatal("live split not replacement")
	}
	assertBalanced(t)
}
func TestReversalNotificationFailureRollsBackReversalStateAndMoney(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")
	record, _, err := f.settlement.Settle(SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID, Lines: []LineInput{{UserID: player.ID, InBase: true}}})
	if err != nil {
		t.Fatal(err)
	}
	before, err := f.ledger.BalanceOfUser(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	const callback = "test:fail_reversal_correction"
	if err := database.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "notifications" {
			tx.AddError(errors.New("correction unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer database.DB.Callback().Create().Remove(callback)
	if _, err := f.settlement.ReverseSettlement(record.ID, "Correction", f.admin.ID); err == nil {
		t.Fatal("reversal committed without correction")
	}
	var stored models.Settlement
	database.DB.First(&stored, "id = ?", record.ID)
	after, _ := f.ledger.BalanceOfUser(player.ID)
	if stored.ReversedAt != nil || after != before {
		t.Fatal("partial reversal committed")
	}
	assertBalanced(t)
}
func TestNudgeRequestRetryConcurrentIdentityAndConflictingTarget(t *testing.T) {
	requireDB(t)
	admin := newUser(t, "admin")
	player := newUser(t, "player")
	other := newUser(t, "other")
	ns := &NotificationService{disabled: true}
	request := uuid.New()
	var group sync.WaitGroup
	results := make(chan *BalanceNudgeResult, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := ns.SendBalanceNudge(context.Background(), player.ID, admin.ID, request)
			results <- result
			errs <- err
		}()
	}
	group.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var id uuid.UUID
	for result := range results {
		if id == uuid.Nil {
			id = result.NotificationID
		}
		if result.NotificationID != id || result.DeliveryState != models.DeliverySuppressed {
			t.Fatal("retry duplicated or replayed paused nudge")
		}
	}
	var count int64
	database.DB.Model(&models.Notification{}).Count(&count)
	if count != 1 {
		t.Fatal("request retry created multiple notices")
	}
	if _, err := ns.SendBalanceNudge(context.Background(), other.ID, admin.ID, request); !errors.Is(err, ErrNudgeRequestConflict) {
		t.Fatalf("request reused for another target: %v", err)
	}
	n := receiptForNudge(t, id)
	if !strings.Contains(n.Body, "Balance checked") || !strings.Contains(n.Body, BalanceWarning(0)) {
		t.Fatal("manual nudge omitted shared wording or check time")
	}
}
func receiptForNudge(t *testing.T, id uuid.UUID) models.Notification {
	t.Helper()
	var n models.Notification
	if err := database.DB.First(&n, "id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	return n
}
func TestNudgeNoLongerLowAtDeliveryIsSuppressed(t *testing.T) {
	requireDB(t)
	admin := newUser(t, "admin")
	player := newUser(t, "player")
	ns := &NotificationService{fcmEnabled: true, fcmClient: &ttlPushClient{}}
	addDeliveryDevice(t, player, "device")
	result, err := ns.SendBalanceNudge(context.Background(), player.ID, admin.ID, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewLedgerService().RecordTopup(CashInput{UserID: player.ID, AmountCents: 2000, CreatedBy: admin.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := ns.ProcessNotificationWork(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	var outbox models.NotificationOutbox
	database.DB.Where("notification_id = ?", result.NotificationID).First(&outbox)
	if outbox.State != models.DeliverySuppressed || outbox.LastErrorCode != "balance_no_longer_low" {
		t.Fatal("paid-up member received stale nudge")
	}
}

func TestSessionChargeCommittedBalanceThresholdBoundaries(t *testing.T) {
	for _, desired := range []int64{-1, 0, 1999, 2000} {
		t.Run(strconv.FormatInt(desired, 10), func(t *testing.T) {
			f := newSettlementFixture(t, 24, 10000)
			player := f.member(t, "boundary")
			input := SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID, Lines: []LineInput{{UserID: player.ID, InBase: true}}}
			preview, err := f.settlement.Preview(input)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.ledger.RecordTopup(CashInput{UserID: player.ID, AmountCents: preview.Totals.ChargedCents + desired, CreatedBy: f.admin.ID}); err != nil {
				t.Fatal(err)
			}
			record, _, err := f.settlement.Settle(input)
			if err != nil {
				t.Fatal(err)
			}
			receipt := receiptFor(t, record.ID, player.ID, models.NotificationSessionCharge)
			facts := moneyFacts(t, receipt)
			if facts["balance_after_cents"] != strconv.FormatInt(desired, 10) || facts["balance_band"] != BalanceNotificationBand(desired) {
				t.Fatalf("wrong committed boundary snapshot: %v", facts)
			}
			if (strings.Contains(receipt.Body, "Please top up.")) != (desired < 2000) {
				t.Fatal("combined balance warning boundary incorrect")
			}
		})
	}
}
func TestConcurrentReversalCreatesOneCorrectionAndOneLedgerReversal(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")
	record, _, err := f.settlement.Settle(SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID, Lines: []LineInput{{UserID: player.ID, InBase: true}}})
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := f.settlement.ReverseSettlement(record.ID, "Correction", f.admin.ID)
			results <- err
		}()
	}
	group.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("reversal succeeded %d times", success)
	}
	var count int64
	database.DB.Model(&models.Notification{}).Where("notification_type = ?", models.NotificationSessionChargeReversed).Count(&count)
	if count != 1 {
		t.Fatalf("created %d correction events", count)
	}
	database.DB.Model(&models.Transaction{}).Where("reverses_transaction_id = ?", record.TransactionID).Count(&count)
	if count != 1 {
		t.Fatalf("created %d ledger reversals", count)
	}
	assertBalanced(t)
}
func TestProviderFailureDoesNotRollBackSessionCharges(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")
	addDeliveryDevice(t, player, "device")
	calls := 0
	ns := &NotificationService{fcmEnabled: true, fcmClient: stubPushClient(func(context.Context, *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
		calls++
		return nil, errors.New("network failed")
	})}
	f.settlement.WithNotifier(ns)
	record, _, err := f.settlement.Settle(SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID, Lines: []LineInput{{UserID: player.ID, InBase: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("provider called inside money transaction")
	}
	before, _ := f.ledger.BalanceOfUser(player.ID)
	if _, err := ns.ProcessNotificationWork(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	after, _ := f.ledger.BalanceOfUser(player.ID)
	if before != after || calls != 1 {
		t.Fatal("provider failure changed posted money")
	}
	var stored models.Settlement
	if err := database.DB.First(&stored, "id = ?", record.ID).Error; err != nil || stored.ReversedAt != nil {
		t.Fatal("provider failure undid settlement")
	}
	assertBalanced(t)
}

func TestNudgeAndSettlementShareAccountsWithoutUserForeignKeyDeadlock(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")
	ns := &NotificationService{disabled: true}
	accountsLocked := make(chan struct{})
	nudgeUserLocked := make(chan struct{})
	var accountSignal, userSignal sync.Once
	const createCallback = "test:settle_wait_nudge_lock"
	const queryCallback = "test:nudge_user_lock_signal"
	if err := database.DB.Callback().Create().Before("gorm:create").Register(createCallback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "charge_lines" {
			accountSignal.Do(func() { close(accountsLocked) })
			select {
			case <-nudgeUserLocked:
			case <-time.After(10 * time.Second):
				tx.AddError(errors.New("nudge lock signal timeout"))
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer database.DB.Callback().Create().Remove(createCallback)
	if err := database.DB.Callback().Query().After("gorm:query").Register(queryCallback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "users" {
			if _, ok := tx.Statement.Clauses["FOR"].Expression.(clause.Locking); ok {
				userSignal.Do(func() { close(nudgeUserLocked) })
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer database.DB.Callback().Query().Remove(queryCallback)
	settlementResult := make(chan error, 1)
	nudgeResult := make(chan error, 1)
	go func() {
		_, _, err := f.settlement.Settle(SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID, Lines: []LineInput{{UserID: player.ID, InBase: true}}})
		settlementResult <- err
	}()
	select {
	case <-accountsLocked:
	case <-time.After(10 * time.Second):
		t.Fatal("settlement did not obtain account locks")
	}
	go func() {
		_, err := ns.SendBalanceNudge(context.Background(), player.ID, f.admin.ID, uuid.New())
		nudgeResult <- err
	}()
	if err := <-settlementResult; err != nil {
		t.Fatal(err)
	}
	if err := <-nudgeResult; err != nil {
		t.Fatal(err)
	}
	assertBalanced(t)
}
