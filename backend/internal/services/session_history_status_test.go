package services

import (
	"testing"

	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

func TestPastSessionsRetainStoredCancellationState(t *testing.T) {
	requireDB(t)
	admin := newUser(t, "history-admin")
	cancelled := historicalSchedule(t, admin.ID, "2024-11-10", "20:00")
	cancelled.Status = models.SessionStatusCancelled
	if err := database.DB.Save(cancelled).Error; err != nil {
		t.Fatal(err)
	}
	played := historicalSchedule(t, admin.ID, "2024-11-12", "20:00")
	history, total, err := NewSettlementService(NewLedgerService()).ListPastSessions(50, 0)
	if err != nil || total != 2 || len(history) != 2 {
		t.Fatalf("history=%+v total=%d error=%v", history, total, err)
	}
	byID := map[string]PastSessionView{}
	for _, row := range history {
		byID[row.SessionID.String()] = row
	}
	if row := byID[cancelled.ID.String()]; row.Status != models.SessionStatusCancelled || row.Settled {
		t.Fatalf("cancelled state lost: %+v", row)
	}
	if row := byID[played.ID.String()]; row.Status == models.SessionStatusCancelled || row.Settled {
		t.Fatalf("uncosted session treated as cancelled: %+v", row)
	}
}
