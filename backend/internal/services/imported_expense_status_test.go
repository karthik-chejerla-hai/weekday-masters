package services

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

func TestExpenseAndAssistantRecognizeImportedSettlements(t *testing.T) {
	settlement, alice := importedHistoryFixture(t)
	expenses := NewExpenseService(settlement, settlement.ledger)
	assistant := NewAssistantService(nil, nil, expenses, settlement.ledger)
	// Match by Sydney play date, including duplicate schedules and a start on
	// the previous UTC day. April 3 is a recording date, not the imported play date.
	early := historicalSchedule(t, alice.ID, "2024-04-02", "00:30")
	duplicate := historicalSchedule(t, alice.ID, "2024-04-02", "20:00")
	unpaid := historicalSchedule(t, alice.ID, "2024-04-03", "20:00")
	cancelled := historicalSchedule(t, alice.ID, "2024-04-05", "20:00")
	if err := database.DB.Model(cancelled).Update("status", models.SessionStatusCancelled).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&models.RSVP{SessionID: unpaid.ID, UserID: alice.ID, Status: models.RSVPStatusIn}).Error; err != nil {
		t.Fatal(err)
	}
	t.Run("outstanding expenses exclude imported games", func(t *testing.T) {
		items, err := expenses.ListUnsettledSessions()
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].SessionID != unpaid.ID || items[0].PlayerCount != 1 {
			t.Fatalf("only the unpaid game should need expenses: %+v", items)
		}
	})
	for _, tc := range []struct {
		period string
		want   map[uuid.UUID]bool
	}{
		{"unsettled", map[uuid.UUID]bool{unpaid.ID: false}},
		{"past", map[uuid.UUID]bool{early.ID: true, duplicate.ID: true, unpaid.ID: false}},
	} {
		t.Run("assistant "+tc.period, func(t *testing.T) {
			assertAssistantSessionStates(t, assistant, tc.period, tc.want)
		})
	}
	for _, session := range []*models.Session{early, duplicate, unpaid} {
		t.Run("assistant session "+session.ID.String(), func(t *testing.T) {
			assertAssistantSessionSettled(t, assistant, session.ID, session.ID != unpaid.ID)
		})
	}
}

func TestExpenseAndAssistantKeepNativeSettlementReversals(t *testing.T) {
	settlement, alice := importedHistoryFixture(t)
	expenses := NewExpenseService(settlement, settlement.ledger)
	assistant := NewAssistantService(nil, nil, expenses, settlement.ledger)
	var liveID, reversedID uuid.UUID
	for _, reverse := range []bool{false, true} {
		session := historicalSchedule(t, alice.ID, "2024-04-03", "20:00")
		record, _, err := settlement.Settle(SettleInput{
			SessionID: session.ID, SettledBy: alice.ID,
			Lines: []LineInput{{UserID: alice.ID, InBase: true}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if reverse {
			if _, err := settlement.ReverseSettlement(record.ID, "Correct native game", alice.ID); err != nil {
				t.Fatal(err)
			}
			reversedID = session.ID
		} else {
			liveID = session.ID
		}
		// An imported date must not take over an existing native settlement,
		// even after reversal. The reversed game must remain available to correct.
		session.SessionDate = time.Date(2024, 4, 2, 0, 0, 0, 0, time.UTC)
		if err := database.DB.Save(session).Error; err != nil {
			t.Fatal(err)
		}
		assertAssistantSessionSettled(t, assistant, session.ID, !reverse)
		if !reverse {
			items, err := expenses.ListUnsettledSessions()
			if err != nil || len(items) != 0 {
				t.Fatalf("live native settlement shown as outstanding: %+v, %v", items, err)
			}
			assertAssistantSessionStates(t, assistant, "past", map[uuid.UUID]bool{session.ID: true})
		}
	}
	items, err := expenses.ListUnsettledSessions()
	if err != nil || len(items) != 1 || items[0].SessionID != reversedID {
		t.Fatalf("reversed native settlement must remain outstanding: %+v, %v", items, err)
	}
	assertAssistantSessionStates(t, assistant, "unsettled", map[uuid.UUID]bool{reversedID: false})
	assertAssistantSessionStates(t, assistant, "past", map[uuid.UUID]bool{liveID: true, reversedID: false})
}

func assertAssistantSessionStates(t *testing.T, service *AssistantService, period string, want map[uuid.UUID]bool) {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"period": period})
	result, err := service.findAssistantSessions(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Sessions []struct {
			ID      uuid.UUID `json:"id"`
			Settled bool      `json:"settled"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	got := map[uuid.UUID]bool{}
	for _, session := range response.Sessions {
		got[session.ID] = session.Settled
	}
	if !reflect.DeepEqual(got, want) || len(response.Sessions) != len(want) {
		t.Fatalf("%s sessions = %v, want %v", period, got, want)
	}
}

func assertAssistantSessionSettled(t *testing.T, service *AssistantService, id uuid.UUID, want bool) {
	t.Helper()
	raw, _ := json.Marshal(map[string]uuid.UUID{"session_id": id})
	result, err := service.getAssistantSession(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.(map[string]any)["settled"]; got != want {
		t.Fatalf("session %s settled = %v, want %v", id, got, want)
	}
}
