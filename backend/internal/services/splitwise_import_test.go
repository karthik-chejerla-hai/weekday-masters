package services

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/splitwise"
)

const importFixture = `Date,Description,Category,Cost,Currency,Alice,Badminton Account,Bob,Former (removed)
2024-04-01,Deposit,General,100.00,AUD,100.00,-100.00,0.00,0.00
2024-04-01,Former deposit,General,5.00,AUD,0.00,-5.00,0.00,5.00
2024-04-03,Game - 2 Apr,General,15.00,AUD,-5.00,15.00,-5.00,-5.00
2024-04-03,Extra hour - 2 Apr,General,4.00,AUD,-2.00,4.00,-2.00,0.00
2024-04-04,Game - 4 Apr,General,9.00,AUD,6.00,0.00,-6.00,0.00
2024-04-04,Total balance, , ,AUD,99.00,-86.00,-13.00,0.00
`

func testImportSource(t *testing.T) (*splitwise.Export, splitwise.Mapping) {
	t.Helper()
	e, err := splitwise.Read(strings.NewReader(importFixture))
	if err != nil {
		t.Fatal(err)
	}
	m := splitwise.Mapping{Members: []splitwise.Member{{SourceName: "Alice", Name: "Alice Example", Email: "alice@example.test"}, {SourceName: "Bob", Name: "Bob Example", Email: "bob@example.test"}}, AdminEmail: "alice@example.test", SessionPayers: map[string]string{"5": "Alice"}}
	return e, m
}

func TestSplitwiseImportReconcilesHistoryAndInactiveParticipants(t *testing.T) {
	requireDB(t)
	source, m := testImportSource(t)
	report, err := ImportSplitwise(source, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.UsersCreated != 2 || report.Transactions != 5 || report.VerifiedCells != 15 || !report.AssetsPending || !report.NotificationsPaused || report.ResidualCents != 0 {
		t.Fatalf("%+v", report)
	}
	var users []models.User
	database.DB.Find(&users)
	if len(users) != 2 {
		t.Fatalf("created %d users", len(users))
	}
	for _, u := range users {
		if u.HasSignedIn() {
			t.Fatal("imported account already claimed")
		}
		got, err := NewLedgerService().BalanceOfUser(u.ID)
		want := int64(9900)
		if u.Email == "bob@example.test" {
			want = -1300
		}
		if err != nil || got != want {
			t.Fatalf("balance %d want %d: %v", got, want, err)
		}
	}
	var old models.SplitwiseParticipant
	database.DB.Where("source_name = ?", "Former (removed)").First(&old)
	if old.UserID != nil || old.ClosingCents != 0 {
		t.Fatal("former participant has a user or balance")
	}
	var rsvps int64
	database.DB.Model(&models.RSVP{}).Count(&rsvps)
	if rsvps != 0 {
		t.Fatal("import created RSVPs")
	}
	history, total, err := NewSettlementService(NewLedgerService()).ListPastSessions(50, 0)
	if err != nil || total != 2 || len(history) != 2 {
		t.Fatalf("history %+v total %d err %v", history, total, err)
	}
	for _, h := range history {
		v, err := NewSettlementService(NewLedgerService()).SettlementForSession(h.SessionID)
		if err != nil || v.Imported == nil {
			t.Fatalf("view %v: %v", v, err)
		}
		if h.ImportedDate == "2024-04-02" {
			if h.TotalCents != 1900 || len(v.Imported.Sources) != 2 {
				t.Fatalf("extra hour was not grouped: %+v", v.Imported)
			}
			found := false
			for _, l := range v.Imported.Lines {
				if l.Inactive {
					found = true
					if l.UserID != nil || l.ChargeCents != 500 {
						t.Fatalf("inactive line: %+v", l)
					}
				}
			}
			if !found {
				t.Fatal("inactive label missing")
			}
		} else {
			for _, l := range v.Imported.Lines {
				if l.Name == "Alice" && (l.PaidCents != 900 || l.ChargeCents != 300 || l.NetCents != 600) {
					t.Fatalf("payer allocation wrong: %+v", l)
				}
			}
		}
	}
	// The persistent pause blocks even a service with provider delivery enabled.
	ns := &NotificationService{emailEnabled: true}
	if err := ns.SendNotification(context.Background(), users[0].ID, models.NotificationBalanceLow, "test", "test", nil); err != nil {
		t.Fatal(err)
	}
	var notifications int64
	database.DB.Model(&models.Notification{}).Count(&notifications)
	if notifications != 0 {
		t.Fatal("pause queued a notification")
	}
	again, err := ImportSplitwise(source, m, nil)
	if err != nil || !again.Reused || again.UsersCreated != 0 {
		t.Fatalf("repeat %+v %v", again, err)
	}
	var count int64
	database.DB.Model(&models.Transaction{}).Count(&count)
	if count != 5 {
		t.Fatal("duplicate transactions")
	}
	assets := &ImportAssets{BankCents: 4000, CourtCreditCents: 3000, ShuttleUnits: 12, ShuttleCents: 2000}
	confirmed, err := ImportSplitwise(source, m, assets)
	if err != nil || confirmed.AssetsPending {
		t.Fatalf("assets %+v %v", confirmed, err)
	}
	position, err := NewLedgerService().Position()
	if err != nil || position.AssetsPending || position.Assets.TotalCents != 9000 || position.SurplusCents != 400 || !position.Balanced {
		t.Fatalf("position %+v %v", position, err)
	}
	if _, err := ImportSplitwise(source, m, assets); err != nil {
		t.Fatal(err)
	}
	assets.BankCents++
	if _, err := ImportSplitwise(source, m, assets); err == nil {
		t.Fatal("changed snapshot accepted")
	}
}

func TestSplitwiseImportAllowsOnlyReviewedReversedTopups(t *testing.T) {
	requireDB(t)
	source, mapping := testImportSource(t)
	alice := models.User{Name: "Alice Existing", Email: mapping.AdminEmail, Auth0ID: "auth0|existing-alice", Role: models.RoleAdmin, MembershipStatus: models.MembershipApproved, IsPlayer: true}
	if err := database.DB.Create(&alice).Error; err != nil {
		t.Fatal(err)
	}
	ledger := NewLedgerService()
	topup, err := ledger.RecordTopup(CashInput{UserID: alice.ID, AmountCents: 5000, CreatedBy: alice.ID})
	if err != nil {
		t.Fatal(err)
	}
	options := SplitwiseImportOptions{AllowReversedTopups: true}
	assets := &ImportAssets{BankCents: 4000, CourtCreditCents: 3000, ShuttleUnits: 12, ShuttleCents: 2000}
	if _, err := ImportSplitwiseWithOptions(source, mapping, assets, options); err == nil {
		t.Fatal("unreversed existing money accepted")
	}
	if _, err := ledger.ReverseTransaction(topup.ID, "Cancel confirmed setup test", alice.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportSplitwise(source, mapping, assets); err == nil {
		t.Fatal("existing ledger accepted without explicit review option")
	}
	report, err := ImportSplitwiseWithOptions(source, mapping, assets, options)
	if err != nil || report.UsersCreated != 1 || report.AssetsPending || report.ResidualCents != 0 {
		t.Fatalf("import after reversal: %+v %v", report, err)
	}
	var retained models.User
	if err := database.DB.First(&retained, "id = ?", alice.ID).Error; err != nil || retained.Auth0ID != alice.Auth0ID {
		t.Fatalf("existing sign-in identity changed: %v", err)
	}
	if balance, err := ledger.BalanceOfUser(alice.ID); err != nil || balance != 9900 {
		t.Fatalf("source balance changed: %d %v", balance, err)
	}
	if _, err := ImportSplitwiseWithOptions(source, mapping, assets, options); err != nil {
		t.Fatal(err)
	}
	var count int64
	database.DB.Model(&models.Transaction{}).Count(&count)
	if count != 8 { // Five source rows, assets, original top-up and its reversal.
		t.Fatalf("setup audit history lost or import duplicated: %d", count)
	}
}

func TestSplitwiseImportRejectsInexactSetupReversal(t *testing.T) {
	requireDB(t)
	source, mapping := testImportSource(t)
	user := newUser(t, "setup-topup")
	ledger := NewLedgerService()
	topup, err := ledger.RecordTopup(CashInput{UserID: user.ID, AmountCents: 5000, CreatedBy: user.ID})
	if err != nil {
		t.Fatal(err)
	}
	reversal, err := ledger.ReverseTransaction(topup.ID, "Setup test", user.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a balanced but incomplete reversal. An overall identity check
	// alone would miss the remaining one cent on both bank and player accounts.
	if err := database.DB.Exec("UPDATE ledger_entries SET amount_cents = amount_cents + 1 WHERE transaction_id = ?", reversal.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ImportSplitwiseWithOptions(source, mapping, nil, SplitwiseImportOptions{AllowReversedTopups: true}); err == nil {
		t.Fatal("inexact reversal accepted")
	}
	var count int64
	database.DB.Model(&models.SplitwiseImport{}).Count(&count)
	if count != 0 {
		t.Fatal("failed preflight wrote an import")
	}
}

func TestSplitwiseImportRefusesAmbiguityAndRollsBack(t *testing.T) {
	requireDB(t)
	source, m := testImportSource(t)
	bad := m
	bad.SessionPayers = nil
	if _, err := ImportSplitwise(source, bad, nil); err == nil {
		t.Fatal("ambiguous payer accepted")
	}
	// Failure after Alice's creation must roll her back.
	bob := models.User{Name: "Bob", Email: "bob@example.test", Auth0ID: "test|bob", MembershipStatus: models.MembershipPending}
	if err := database.DB.Create(&bob).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ImportSplitwise(source, m, nil); err == nil {
		t.Fatal("pending identity accepted")
	}
	var n int64
	database.DB.Model(&models.User{}).Count(&n)
	if n != 1 {
		t.Fatal("partial user import")
	}
	database.DB.Model(&models.Transaction{}).Count(&n)
	if n != 0 {
		t.Fatal("partial ledger import")
	}
	database.DB.Model(&models.SplitwiseImport{}).Count(&n)
	if n != 0 {
		t.Fatal("partial batch")
	}
}

func TestSplitwiseImportRejectsTamperingAndConcurrentDuplicates(t *testing.T) {
	requireDB(t)
	source, m := testImportSource(t)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := ImportSplitwise(source, m, nil); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var n int64
	database.DB.Model(&models.Transaction{}).Count(&n)
	if n != 5 {
		t.Fatalf("concurrent duplicate count %d", n)
	}
	var change models.SplitwiseChange
	database.DB.Where("charge_cents > 0").First(&change)
	if err := database.DB.Model(&change).Update("charge_cents", 123).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ImportSplitwise(source, m, nil); err == nil {
		t.Fatal("source charge tampering undetected")
	}
}

func TestNotificationsDisabledSkipsProvidersAndDatabase(t *testing.T) {
	ns := NewNotificationService(NotificationConfig{Disabled: true, FirebaseCredentials: "invalid", SendGridAPIKey: "unused"})
	if ns.IsEnabled() || ns.fcmClient != nil || ns.sendGridClient != nil {
		t.Fatal("disabled service initialized providers")
	}
	if err := ns.SendNotification(context.Background(), uuid.New(), models.NotificationBalanceLow, "ignored", "ignored", nil); err != nil {
		t.Fatal(err)
	}
}

// Opt-in test: real data stays outside version control. The comparison below
// uses encoding/csv directly and SQL, independently of the importer verifier.
func TestSplitwisePrivateExportReconciliation(t *testing.T) {
	path, mappingPath := os.Getenv("RALLY_SPLITWISE_CSV"), os.Getenv("RALLY_SPLITWISE_MAPPING")
	if path == "" || mappingPath == "" {
		t.Skip("private source paths not supplied")
	}
	requireDB(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	e, err := splitwise.Read(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	mappingData, err := os.ReadFile(mappingPath)
	if err != nil {
		t.Fatal(err)
	}
	var m splitwise.Mapping
	if err := json.Unmarshal(mappingData, &m); err != nil {
		t.Fatal(err)
	}
	r, err := ImportSplitwise(e, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	rowNumber := 0
	for _, row := range rows[1:] {
		if len(row) < 5 || strings.TrimSpace(strings.Join(row, "")) == "" {
			continue
		}
		if row[1] == "Total balance" {
			continue
		}
		rowNumber++
		for col, name := range rows[0][5:] {
			if name == splitwise.ClubName {
				continue
			}
			want, err := strconv.ParseInt(strings.ReplaceAll(row[col+5], ".", ""), 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			var got int64
			err = database.DB.Raw(`SELECT COALESCE(SUM(e.amount_cents),0) FROM splitwise_records r JOIN splitwise_participants p ON p.import_id=r.import_id LEFT JOIN ledger_entries e ON e.transaction_id=r.transaction_id AND e.account_id=p.account_id WHERE r.import_id=? AND r.row_number=? AND p.source_name=?`, r.ImportID, rowNumber, name).Scan(&got).Error
			if err != nil || got != want {
				t.Fatalf("row %d column %s: got %d want %d (%v)", rowNumber, name, got, want, err)
			}
		}
	}
	// Independently check every gross session share against the source net
	// and the reviewed payer, and each grouped history view against its lines.
	for _, row := range e.Rows {
		if !row.IsSession {
			continue
		}
		payer := splitwise.ClubName
		if name := m.SessionPayers[strconv.Itoa(row.Number)]; name != "" {
			payer = name
		}
		var allocated int64
		for col, name := range e.Names {
			if col == e.ClubIndex {
				continue
			}
			want := -row.Amounts[col]
			if name == payer {
				want += row.CostCents
			}
			var got int64
			err := database.DB.Raw(`SELECT c.charge_cents FROM splitwise_changes c JOIN splitwise_records r ON r.id=c.record_id JOIN splitwise_participants p ON p.id=c.participant_id WHERE r.import_id=? AND r.row_number=? AND p.source_name=?`, r.ImportID, row.Number, name).Scan(&got).Error
			if err != nil || got != want {
				t.Fatalf("row %d share for %s: got %d want %d (%v)", row.Number, name, got, want, err)
			}
			allocated += got
		}
		if allocated != row.CostCents {
			t.Fatalf("row %d cost differs from allocations", row.Number)
		}
	}
	history, total, err := NewSettlementService(NewLedgerService()).ListPastSessions(200, 0)
	if err != nil || total != int64(r.Sessions) || len(history) != r.Sessions {
		t.Fatalf("history count %d versus %d (%v)", total, r.Sessions, err)
	}
	for _, session := range history {
		view, err := NewSettlementService(NewLedgerService()).SettlementForSession(session.SessionID)
		if err != nil || view == nil || view.Imported == nil {
			t.Fatalf("missing session view: %v", err)
		}
		var sum int64
		for _, line := range view.Imported.Lines {
			sum += line.ChargeCents
		}
		if sum != session.TotalCents || sum != view.Imported.TotalCents {
			t.Fatal("displayed history and charges disagree")
		}
	}
	var n int64
	database.DB.Model(&models.User{}).Count(&n)
	if n != int64(len(m.Members)) {
		t.Fatalf("%d users instead of %d", n, len(m.Members))
	}
	var notifications int64
	database.DB.Model(&models.Notification{}).Count(&notifications)
	if notifications != 0 {
		t.Fatal("notifications created")
	}
	if _, err := ImportSplitwise(e, m, nil); err != nil {
		t.Fatal(err)
	}
	t.Logf("Verified %d transactions, %d source cells, %d current users, %d inactive participants; residual %d cents", rowNumber, r.VerifiedCells, n, len(e.Names)-1-len(m.Members), r.ResidualCents)
}
