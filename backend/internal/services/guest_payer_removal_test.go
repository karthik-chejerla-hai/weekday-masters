package services

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

func TestRemoveGuestPayerRequiresReplacement(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	payer := f.member(t, "payer")
	replacement := f.member(t, "replacement")
	rs := NewRSVPService(nil)
	us := NewUserService("").WithRSVPs(rs)
	session := newSession(t, f.sessions, f.admin.ID, 1)
	rsvpIn(t, rs, session.ID, payer.ID)
	guest := addGuest(t, rs, session.ID, payer.ID, "Alex")
	second := addGuest(t, rs, session.ID, payer.ID, "Sam")

	assertBlocked := func(count string) {
		t.Helper()
		_, err := us.RemoveMember(payer.ID, f.admin.ID)
		if err == nil || !strings.Contains(err.Error(), count+" confirmed guest") || !strings.Contains(err.Error(), "Assign a replacement payer") {
			t.Fatalf("expected replacement payer instruction for %s guests, got %v", count, err)
		}
		var saved models.User
		if err := database.DB.First(&saved, "id = ?", payer.ID).Error; err != nil || !saved.IsApproved() {
			t.Fatalf("blocked removal changed membership: %v %+v", err, saved)
		}
		if statusOf(t, rs, session.ID, payer.ID).Status != models.RSVPStatusIn {
			t.Fatal("blocked removal released the member's place")
		}
	}
	assertBlocked("2")
	for i, g := range []*models.GuestRSVP{guest, second} {
		if _, err := rs.SaveGuestRSVP(GuestRSVPInput{
			ID: g.ID, SessionID: session.ID, GuestName: g.GuestName,
			ChargedToUserID: replacement.ID, Status: models.RSVPStatusIn,
		}); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			assertBlocked("1")
		}
	}
	if _, err := us.RemoveMember(payer.ID, f.admin.ID); err != nil {
		t.Fatalf("replacement should permit removal: %v", err)
	}
	if _, err := rs.GetUserRSVPForSession(session.ID, payer.ID); err == nil {
		t.Fatal("successful removal retained the member's place")
	}
	if err := database.DB.Model(&models.Session{}).Where("id = ?", session.ID).Updates(map[string]any{
		"starts_at": time.Now().Add(-4 * time.Hour), "ends_at": time.Now().Add(-2 * time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	_, preview, err := f.settlement.Settle(SettleInput{SessionID: session.ID, SettledBy: f.admin.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Lines) != 2 {
		t.Fatalf("expected both guest charges, got %+v", preview.Lines)
	}
	for _, line := range preview.Lines {
		if line.UserID != replacement.ID || line.GuestName == "" {
			t.Fatalf("guest charge lost its replacement payer or name: %+v", line)
		}
	}
	if balance, err := f.ledger.BalanceOfUser(payer.ID); err != nil || balance != 0 {
		t.Fatalf("removed payer acquired debt: balance=%d err=%v", balance, err)
	}
	if balance, err := f.ledger.BalanceOfUser(replacement.ID); err != nil || balance != -preview.Totals.ChargedCents {
		t.Fatalf("replacement did not receive the guest charges: balance=%d err=%v", balance, err)
	}
	assertBalanced(t)
}

func TestRemoveGuestPayerSessionStates(t *testing.T) {
	for _, state := range []string{"upcoming", "past unsettled", "reversed", "out", "cancelled", "settled"} {
		t.Run(state, func(t *testing.T) {
			f := newSettlementFixture(t, 24, 10000)
			payer := f.member(t, "payer")
			rs := NewRSVPService(nil)
			session := newSession(t, f.sessions, f.admin.ID, 1)
			guest := addGuest(t, rs, session.ID, payer.ID, "Alex")
			switch state {
			case "out":
				if _, err := rs.SaveGuestRSVP(GuestRSVPInput{ID: guest.ID, SessionID: session.ID, GuestName: "Alex", ChargedToUserID: payer.ID, Status: models.RSVPStatusOut}); err != nil {
					t.Fatal(err)
				}
			case "cancelled":
				if err := database.DB.Model(&models.Session{}).Where("id = ?", session.ID).Update("status", models.SessionStatusCancelled).Error; err != nil {
					t.Fatal(err)
				}
			case "past unsettled", "reversed", "settled":
				if err := database.DB.Model(&models.Session{}).Where("id = ?", session.ID).Updates(map[string]any{"starts_at": time.Now().Add(-4 * time.Hour), "ends_at": time.Now().Add(-2 * time.Hour)}).Error; err != nil {
					t.Fatal(err)
				}
				if state != "past unsettled" {
					settled, preview, err := f.settlement.Settle(SettleInput{SessionID: session.ID, SettledBy: f.admin.ID})
					if err != nil {
						t.Fatal(err)
					}
					if state == "reversed" {
						if _, err := f.settlement.ReverseSettlement(settled.ID, "correct attendance", f.admin.ID); err != nil {
							t.Fatal(err)
						}
					} else if _, err := f.ledger.RecordTopup(CashInput{UserID: payer.ID, AmountCents: preview.Totals.ChargedCents, CreatedBy: f.admin.ID}); err != nil {
						t.Fatal(err)
					}
				}
			}
			_, err := NewUserService("").WithRSVPs(rs).RemoveMember(payer.ID, f.admin.ID)
			wantBlocked := state == "upcoming" || state == "past unsettled" || state == "reversed"
			if wantBlocked && (err == nil || !strings.Contains(err.Error(), "replacement payer")) {
				t.Fatalf("expected guest obligation to block removal, got %v", err)
			}
			if !wantBlocked && err != nil {
				t.Fatalf("guest history should not block removal: %v", err)
			}
		})
	}
}

func TestConcurrentGuestSaveAndPayerRemoval(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")
	us := NewUserService("").WithRSVPs(rs)
	for i := 0; i < 12; i++ {
		payer := newUser(t, "payer")
		session := newSession(t, ss, admin.ID, 1)
		rsvpIn(t, rs, session.ID, payer.ID)
		in := GuestRSVPInput{SessionID: session.ID, GuestName: "Alex", ChargedToUserID: payer.ID, Status: models.RSVPStatusIn}
		// Exercise both a new booking and reassignment of an existing booking.
		if i%2 == 0 {
			in.ID = addGuest(t, rs, session.ID, admin.ID, "Alex").ID
		}
		start := make(chan struct{})
		var wg sync.WaitGroup
		var guestErr, removeErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, guestErr = rs.SaveGuestRSVP(in)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, removeErr = us.RemoveMember(payer.ID, admin.ID)
		}()
		close(start)
		wg.Wait()
		var saved models.User
		if err := database.DB.First(&saved, "id = ?", payer.ID).Error; err != nil {
			t.Fatal(err)
		}
		var guests int64
		if err := database.DB.Model(&models.GuestRSVP{}).Where("charged_to_user_id = ? AND status = ?", payer.ID, models.RSVPStatusIn).Count(&guests).Error; err != nil {
			t.Fatal(err)
		}
		if removeErr == nil {
			if saved.MembershipStatus != models.MembershipRemoved || guests != 0 || guestErr == nil || !strings.Contains(guestErr.Error(), "approved member") {
				t.Fatalf("removed payer retained a guest: member=%s guests=%d guestErr=%v", saved.MembershipStatus, guests, guestErr)
			}
		} else if !saved.IsApproved() || guests != 1 || guestErr != nil || !strings.Contains(removeErr.Error(), "replacement payer") {
			t.Fatalf("unexpected concurrent result: member=%s guests=%d guestErr=%v removeErr=%v", saved.MembershipStatus, guests, guestErr, removeErr)
		}
	}
}
