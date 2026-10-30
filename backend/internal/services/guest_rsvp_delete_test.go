package services

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

func TestDeleteGuestRSVPReleasesPlaceAndPayer(t *testing.T) {
	rs, ss, notifier := newTestServices(t)
	payer := newUser(t, "payer")
	admin := newUser(t, "admin")
	session := newSession(t, ss, admin.ID, 1)
	players := newUsers(t, 7)
	for _, player := range players[:5] {
		rsvpIn(t, rs, session.ID, player.ID)
	}
	guest := addGuest(t, rs, session.ID, payer.ID, "Rondo")
	for _, player := range players[5:] {
		rsvpIn(t, rs, session.ID, player.ID)
	}
	notifier.reset()
	if err := rs.DeleteGuestRSVP(session.ID, guest.ID); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := database.DB.Model(&models.GuestRSVP{}).Where("id = ?", guest.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("guest not removed: %d %v", count, err)
	}
	if got := summaryOf(t, rs, session.ID); got.TotalIn != 6 || got.TotalWaitlisted != 1 || got.TotalOut != 0 {
		t.Fatalf("wrong attendance: %+v", got)
	}
	if statusOf(t, rs, session.ID, players[5].ID).Status != models.RSVPStatusIn || statusOf(t, rs, session.ID, players[6].ID).Status != models.RSVPStatusWaitlisted {
		t.Fatal("wrong waitlist promotion")
	}
	if users := notifier.notifiedUsers(); len(users) != 1 || users[0] != players[5].ID {
		t.Fatalf("wrong notifications: %v", users)
	}
	lines, err := NewSettlementService(NewLedgerService()).defaultLines(session.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		if line.GuestName != "" || line.UserID == payer.ID {
			t.Fatalf("removed guest retained a charge: %+v", line)
		}
	}
	if _, err := NewUserService("").RemoveMember(payer.ID, admin.ID); err != nil {
		t.Fatalf("deleted guest still blocks payer removal: %v", err)
	}
	if err := database.DB.Model(&models.LedgerEntry{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("removal moved money: %d %v", count, err)
	}
}

func TestDeleteOutGuestAllowsNameReuse(t *testing.T) {
	rs, ss, notifier := newTestServices(t)
	payer := newUser(t, "payer")
	session := newSession(t, ss, payer.ID, 1)
	guest, err := rs.SaveGuestRSVP(GuestRSVPInput{SessionID: session.ID, GuestName: "Rondo", ChargedToUserID: payer.ID, Status: models.RSVPStatusOut})
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.DeleteGuestRSVP(session.ID, guest.ID); err != nil {
		t.Fatal(err)
	}
	if got := summaryOf(t, rs, session.ID); got.TotalOut != 0 || got.TotalIn != 0 || got.SpotsLeft != 6 {
		t.Fatalf("wrong attendance: %+v", got)
	}
	if len(notifier.notifiedUsers()) != 0 {
		t.Fatal("OUT removal sent a promotion")
	}
	replacement := addGuest(t, rs, session.ID, payer.ID, "Rondo")
	if replacement.ID == guest.ID {
		t.Fatal("old RSVP was retained")
	}
}

func TestDeleteGuestRSVPGuardsHistoryAndSessionScope(t *testing.T) {
	for _, state := range []string{"wrong session", "missing guest", "closed", "cancelled", "started", "settled", "reversed"} {
		t.Run(state, func(t *testing.T) {
			f := newSettlementFixture(t, 24, 10000)
			rs := NewRSVPService(nil)
			payer := f.member(t, "payer")
			session := newSession(t, f.sessions, f.admin.ID, 1)
			guest := addGuest(t, rs, session.ID, payer.ID, "Rondo")
			sessionID, guestID := session.ID, guest.ID
			switch state {
			case "wrong session":
				sessionID = newSession(t, f.sessions, f.admin.ID, 1).ID
			case "missing guest":
				guestID = uuid.New()
			case "closed", "cancelled":
				if err := database.DB.Model(&models.Session{}).Where("id = ?", session.ID).Update("status", state).Error; err != nil {
					t.Fatal(err)
				}
			case "started", "settled", "reversed":
				if err := database.DB.Model(&models.Session{}).Where("id = ?", session.ID).Updates(map[string]any{"starts_at": time.Now().Add(-4 * time.Hour), "ends_at": time.Now().Add(-2 * time.Hour)}).Error; err != nil {
					t.Fatal(err)
				}
				if state != "started" {
					settled, _, err := f.settlement.Settle(SettleInput{SessionID: session.ID, SettledBy: f.admin.ID})
					if err != nil {
						t.Fatal(err)
					}
					if state == "reversed" {
						if _, err := f.settlement.ReverseSettlement(settled.ID, "correction", f.admin.ID); err != nil {
							t.Fatal(err)
						}
					}
					// Even moving the session back into the future cannot erase settlement history.
					if err := database.DB.Model(&models.Session{}).Where("id = ?", session.ID).Update("starts_at", time.Now().Add(24*time.Hour)).Error; err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := rs.DeleteGuestRSVP(sessionID, guestID); err == nil {
				t.Fatal("invalid removal succeeded")
			}
			var saved models.GuestRSVP
			if err := database.DB.First(&saved, "id = ?", guest.ID).Error; err != nil {
				t.Fatalf("guest history was lost: %v", err)
			}
		})
	}
}

func TestConcurrentGuestRemovalPromotesOnlyOnce(t *testing.T) {
	rs, ss, notifier := newTestServices(t)
	payer := newUser(t, "payer")
	session := newSession(t, ss, payer.ID, 1)
	players := newUsers(t, 7)
	for _, player := range players[:5] {
		rsvpIn(t, rs, session.ID, player.ID)
	}
	guest := addGuest(t, rs, session.ID, payer.ID, "Rondo")
	for _, player := range players[5:] {
		rsvpIn(t, rs, session.ID, player.ID)
	}
	notifier.reset()
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) { defer wg.Done(); <-start; errs[i] = rs.DeleteGuestRSVP(session.ID, guest.ID) }(i)
	}
	close(start)
	wg.Wait()
	if (errs[0] == nil) == (errs[1] == nil) {
		t.Fatalf("expected one removal: %v", errs)
	}
	if got := summaryOf(t, rs, session.ID); got.TotalIn != 6 || got.TotalWaitlisted != 1 {
		t.Fatalf("wrong attendance: %+v", got)
	}
	if users := notifier.notifiedUsers(); len(users) != 1 || users[0] != players[5].ID {
		t.Fatalf("duplicate promotion: %v", users)
	}
}

func TestDeleteLegacyDuplicateGuestKeepsOtherBooking(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	payer := newUser(t, "payer")
	otherPayer := newUser(t, "other")
	session := newSession(t, ss, payer.ID, 1)
	keep := addGuest(t, rs, session.ID, payer.ID, "Rondo")
	// Simulate a duplicate created before name validation was introduced.
	duplicate := models.GuestRSVP{SessionID: session.ID, GuestName: "Rondo", ChargedToUserID: otherPayer.ID, Status: models.RSVPStatusIn}
	if err := database.DB.Create(&duplicate).Error; err != nil {
		t.Fatal(err)
	}
	if err := rs.DeleteGuestRSVP(session.ID, duplicate.ID); err != nil {
		t.Fatal(err)
	}
	detail, err := ss.GetSessionByID(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.GuestRSVPs) != 1 || detail.GuestRSVPs[0].ID != keep.ID || detail.GuestRSVPs[0].ChargedToUserID != payer.ID {
		t.Fatalf("removed the wrong booking: %+v", detail.GuestRSVPs)
	}
	if got := summaryOf(t, rs, session.ID); got.TotalIn != 1 || got.SpotsLeft != 5 {
		t.Fatalf("duplicate still uses a place: %+v", got)
	}
}
