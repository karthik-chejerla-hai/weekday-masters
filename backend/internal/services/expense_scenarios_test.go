package services

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
)

// Each case starts after a real previous expense, then a purchase at a different
// unit cost. Expected cents are worked examples, not calls to the cost calculator.
func TestExpenseRequestedScenariosAfterPreviousExpense(t *testing.T) {
	for _, tc := range []struct {
		name                                                         string
		hours, players, shuttles                                     int
		earlyLeaver                                                  bool
		courtCost, shuttleCost, charged, courtAfter, stockValueAfter int64
		stockUnitsAfter                                              int
	}{
		{"regular_2h_6_confirmed", 2, 6, 8, false, 6000, 3940, 9940, 38000, 10343, 21},
		{"regular_2h_5_confirmed", 2, 5, 8, false, 6000, 3940, 9940, 38000, 10343, 21},
		{"extended_3h_5_confirmed", 3, 5, 10, false, 8300, 4925, 13225, 35700, 9358, 19},
		{"extended_3h_6_confirmed_one_leaves_after_2h", 3, 6, 10, true, 8300, 4925, 13225, 35700, 9358, 19},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSettlementFixture(t, 24, 10000)
			f.admin.Role = models.RoleAdmin
			expenses := NewExpenseService(f.settlement, f.ledger)
			previousPlayer := f.member(t, "previous-player")
			previousUnits := 7
			previousPreview, err := expenses.Preview(f.session.ID, ExpenseInput{TotalHours: 2, ShuttlesUsed: &previousUnits, ParticipantIDs: []uuid.UUID{previousPlayer.ID}}, &f.admin)
			if err != nil {
				t.Fatal(err)
			}
			previousInput := previousPreview.Input
			previousInput.ExpectedPreview = previousPreview.Fingerprint
			if _, err := expenses.Confirm(f.session.ID, previousInput, &f.admin); err != nil {
				t.Fatal(err)
			}
			previousPosition, err := f.ledger.Position()
			if err != nil {
				t.Fatal(err)
			}
			if previousPosition.Assets.CourtCreditCents != 44000 || previousPosition.Assets.ShuttleStockUnits != 17 || previousPosition.Assets.ShuttleStockCents != 7083 {
				t.Fatalf("wrong starting state after previous expense: %+v", previousPosition.Assets)
			}
			if _, err := f.ledger.RecordShuttlePurchase(AssetPurchaseInput{Units: 12, AmountCents: 7200, CreatedBy: f.admin.ID}); err != nil {
				t.Fatal(err)
			}
			before, err := f.ledger.Position()
			if err != nil {
				t.Fatal(err)
			}
			if before.Assets.ShuttleStockUnits != 29 || before.Assets.ShuttleStockCents != 14283 {
				t.Fatalf("purchase did not blend stock: %+v", before.Assets)
			}
			session, err := f.sessions.CreateSession(CreateSessionInput{Title: tc.name, SessionDate: utils.NowInSydney().AddDate(0, 0, -1), StartTime: "18:00", EndTime: "20:00", Courts: 1, CreatedBy: f.admin.ID})
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]uuid.UUID, tc.players)
			rsvps := NewRSVPService(nil)
			for i := range ids {
				member := f.member(t, fmt.Sprintf("participant-%d", i+1))
				ids[i] = member.ID
				if _, err := rsvps.CreateOrUpdateRSVP(RSVPInput{SessionID: session.ID, UserID: member.ID, Status: models.RSVPStatusIn}, true); err != nil {
					t.Fatal(err)
				}
			}
			// Use the public JSON shape so this test also detects a silently ignored
			// extra-hour group. Default main participants must come from the RSVPs.
			body := map[string]any{"total_hours": tc.hours, "shuttles_used": tc.shuttles}
			if tc.earlyLeaver {
				body["extra_participant_ids"] = ids[:5]
			}
			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			var input ExpenseInput
			if err := json.Unmarshal(raw, &input); err != nil {
				t.Fatal(err)
			}
			preview, err := expenses.Preview(session.ID, input, &f.admin)
			if err != nil {
				t.Fatal(err)
			}
			if len(preview.Input.ParticipantIDs) != tc.players {
				t.Fatal("confirmed RSVPs were not selected")
			}
			if got := preview.Settlement.Totals; got.CourtCents != tc.courtCost || got.ShuttleCents != tc.shuttleCost || got.ShuttleUnits != tc.shuttles || got.ChargedCents != tc.charged || got.SurplusCents != 0 {
				t.Fatalf("wrong expense totals: %+v", got)
			}
			if preview.CourtCreditAfterCents != tc.courtAfter || preview.Settlement.StockAfter.Units != tc.stockUnitsAfter || preview.Settlement.StockAfter.AmountCents != tc.stockValueAfter {
				t.Fatalf("wrong asset preview: %+v", preview)
			}
			if tc.hours == 3 {
				if preview.Settlement.Bands["base"].ShuttleCents != 3283 || preview.Settlement.Bands["extra"].ShuttleCents != 1642 {
					t.Fatalf("shuttle value must split 2:1 by hours: %+v", preview.Settlement.Bands)
				}
			}
			var sum int64
			for _, line := range preview.Settlement.Lines {
				sum += line.AmountCents
				if tc.earlyLeaver {
					if line.UserID == ids[5] {
						if line.InExtra || line.AmountCents < 1547 || line.AmountCents > 1548 {
							t.Fatalf("early leaver paid for the extra hour: %+v", line)
						}
					} else if !line.InExtra || line.AmountCents < 2335 || line.AmountCents > 2337 {
						t.Fatalf("wrong full-session share: %+v", line)
					}
				} else if line.AmountCents < tc.charged/int64(tc.players) || line.AmountCents > (tc.charged+int64(tc.players)-1)/int64(tc.players) {
					t.Fatalf("unequal whole-session share: %+v", line)
				}
			}
			if sum != tc.charged {
				t.Fatalf("shares sum to %d, expected %d", sum, tc.charged)
			}
			unchanged, err := f.ledger.Position()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(unchanged.Assets, before.Assets) {
				t.Fatal("preview changed club assets")
			}
			input = preview.Input
			input.ExpectedPreview = preview.Fingerprint
			record, err := expenses.Confirm(session.ID, input, &f.admin)
			if err != nil {
				t.Fatal(err)
			}
			after, err := f.ledger.Position()
			if err != nil {
				t.Fatal(err)
			}
			if after.Assets.CourtCreditCents != tc.courtAfter || after.Assets.ShuttleStockUnits != tc.stockUnitsAfter || after.Assets.ShuttleStockCents != tc.stockValueAfter {
				t.Fatalf("wrong persisted assets: %+v", after.Assets)
			}
			if after.Assets.BankCents != before.Assets.BankCents {
				t.Fatal("settlement changed bank funds")
			}
			for _, line := range preview.Settlement.Lines {
				balance, err := f.ledger.BalanceOfUser(line.UserID)
				if err != nil || balance != -line.AmountCents {
					t.Fatalf("wrong member balance: %d, charge %d, error %v", balance, line.AmountCents, err)
				}
			}
			if balance, err := f.ledger.BalanceOfUser(previousPlayer.ID); err != nil || balance != -8917 {
				t.Fatalf("previous expense changed: %d %v", balance, err)
			}
			view, err := f.settlement.SettlementForSession(session.ID)
			if err != nil || view.Totals != preview.Settlement.Totals {
				t.Fatalf("history differs from preview: %+v %v", view, err)
			}
			if tc.earlyLeaver && view.Bands["extra"].Heads != 5 {
				t.Fatal("history lost the extra-hour group")
			}
			var count int64
			if err := database.DB.Model(&models.Settlement{}).Where("session_id = ?", session.ID).Count(&count).Error; err != nil || count != 1 {
				t.Fatalf("settlement count %d: %v", count, err)
			}
			if _, err := expenses.Confirm(session.ID, input, &f.admin); err == nil {
				t.Fatal("duplicate expense accepted")
			}
			assertBalanced(t)
			if _, err := f.settlement.ReverseSettlement(record.ID, "scenario test reversal", f.admin.ID); err != nil {
				t.Fatal(err)
			}
			restored, err := f.ledger.Position()
			if err != nil || restored.Assets.CourtCreditCents != 44000 || restored.Assets.ShuttleStockUnits != 29 || restored.Assets.ShuttleStockCents != 14283 {
				t.Fatalf("reversal did not restore the previous expense state: %+v %v", restored, err)
			}
			assertBalanced(t)
		})
	}
}
