package services

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

func addGuest(t *testing.T, rs *RSVPService, sessionID, payerID uuid.UUID, name string) *models.GuestRSVP {
	t.Helper()
	guest, err := rs.SaveGuestRSVP(GuestRSVPInput{SessionID: sessionID, GuestName: name, ChargedToUserID: payerID, Status: models.RSVPStatusIn})
	if err != nil {
		t.Fatal(err)
	}
	return guest
}

func TestGuestRSVPCapacityAndPromotion(t *testing.T) {
	rs, ss, notifier := newTestServices(t)
	payer := newUser(t, "payer")
	session := newSession(t, ss, payer.ID, 1)
	players := newUsers(t, 6)
	for _, player := range players[:5] {
		rsvpIn(t, rs, session.ID, player.ID)
	}
	guest := addGuest(t, rs, session.ID, payer.ID, "  Alex  ")
	if guest.GuestName != "Alex" || guest.ChargedToUser.ID != payer.ID {
		t.Fatalf("bad guest: %+v", guest)
	}
	if got := rsvpIn(t, rs, session.ID, players[5].ID); got.Status != models.RSVPStatusWaitlisted {
		t.Fatal("returning member displaced guest")
	}
	if _, err := rs.SaveGuestRSVP(GuestRSVPInput{SessionID: session.ID, GuestName: "Overflow", ChargedToUserID: payer.ID, Status: models.RSVPStatusIn}); err == nil {
		t.Fatal("overbooked guest")
	}
	if _, err := rs.GetUserRSVPForSession(session.ID, payer.ID); err == nil {
		t.Fatal("guest created a payer RSVP")
	}
	if got := summaryOf(t, rs, session.ID); got.TotalIn != 6 || got.SpotsLeft != 0 {
		t.Fatalf("bad summary: %+v", got)
	}
	// Editing a confirmed guest at capacity must retain the place.
	in := GuestRSVPInput{ID: guest.ID, SessionID: session.ID, GuestName: "Alex Smith", ChargedToUserID: payer.ID, Status: models.RSVPStatusIn}
	if _, err := rs.SaveGuestRSVP(in); err != nil {
		t.Fatal(err)
	}
	notifier.reset()
	in.Status = models.RSVPStatusOut
	if _, err := rs.SaveGuestRSVP(in); err != nil {
		t.Fatal(err)
	}
	if got := statusOf(t, rs, session.ID, players[5].ID); got.Status != models.RSVPStatusIn {
		t.Fatal("guest did not release place")
	}
	if got := notifier.notifiedUsers(); len(got) != 1 || got[0] != players[5].ID {
		t.Fatalf("bad notifications: %v", got)
	}
	if got := summaryOf(t, rs, session.ID); got.TotalIn != 6 || got.TotalOut != 1 {
		t.Fatalf("bad summary: %+v", got)
	}
}

func TestGuestRSVPVisibilityAndSettlementDefaults(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	payer := newUser(t, "payer")
	session := newSession(t, ss, payer.ID, 1)
	rsvpIn(t, rs, session.ID, payer.ID)
	guest := addGuest(t, rs, session.ID, payer.ID, "Alex")
	addGuest(t, rs, session.ID, payer.ID, "Sam")
	if _, err := rs.CreateOrUpdateRSVP(RSVPInput{SessionID: session.ID, UserID: payer.ID, Status: models.RSVPStatusOut}, true); err != nil {
		t.Fatal(err)
	}
	detail, err := ss.GetSessionByID(session.ID)
	if err != nil || len(detail.GuestRSVPs) != 2 || detail.GuestRSVPs[0].GuestName != guest.GuestName {
		t.Fatalf("missing guest detail: %v %+v", err, detail)
	}
	upcoming, err := ss.ListUpcomingSessions()
	if err != nil || len(upcoming) != 1 || len(upcoming[0].GuestRSVPs) != 2 {
		t.Fatalf("missing guest list: %v %+v", err, upcoming)
	}
	lines, err := NewSettlementService(NewLedgerService()).defaultLines(session.ID, true)
	if err != nil || len(lines) != 2 {
		t.Fatalf("bad lines: %v %+v", err, lines)
	}
	for _, line := range lines {
		if line.UserID != payer.ID || line.GuestName == "" || !line.InBase || !line.InExtra {
			t.Fatalf("bad guest line: %+v", line)
		}
	}
	if err := ss.DeleteSession(session.ID); err == nil {
		t.Fatal("deleted session with guests")
	}
}

func TestGuestRSVPValidationAndClosedSessions(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	payer := newUser(t, "payer")
	session := newSession(t, ss, payer.ID, 1)
	valid := GuestRSVPInput{SessionID: session.ID, GuestName: "Alex", ChargedToUserID: payer.ID, Status: models.RSVPStatusIn}
	for _, modify := range []func(*GuestRSVPInput){
		func(in *GuestRSVPInput) { in.GuestName = "  " },
		func(in *GuestRSVPInput) { in.GuestName = strings.Repeat("x", 256) },
		func(in *GuestRSVPInput) { in.Status = models.RSVPStatusMaybe },
		func(in *GuestRSVPInput) { in.ChargedToUserID = uuid.New() },
		func(in *GuestRSVPInput) { in.ID = uuid.New() },
		func(in *GuestRSVPInput) { in.SessionID = uuid.New() },
	} {
		in := valid
		modify(&in)
		if _, err := rs.SaveGuestRSVP(in); err == nil {
			t.Fatalf("accepted invalid input: %+v", in)
		}
	}
	guest := addGuest(t, rs, session.ID, payer.ID, "Alex")
	other := newSession(t, ss, payer.ID, 1)
	wrongSession := valid
	wrongSession.ID = guest.ID
	wrongSession.SessionID = other.ID
	if _, err := rs.SaveGuestRSVP(wrongSession); err == nil {
		t.Fatal("edited guest from another session")
	}
	for _, state := range []models.SessionStatus{models.SessionStatusCancelled, models.SessionStatusClosed} {
		database.DB.Model(&models.Session{}).Where("id = ?", session.ID).Update("status", state)
		if _, err := rs.SaveGuestRSVP(valid); err == nil {
			t.Fatalf("accepted %s session", state)
		}
	}
	database.DB.Model(&models.Session{}).Where("id = ?", session.ID).Updates(map[string]any{"status": models.SessionStatusOpen, "starts_at": time.Now().Add(-time.Hour)})
	if _, err := rs.SaveGuestRSVP(valid); err == nil {
		t.Fatal("accepted started session")
	}
	database.DB.Model(&models.User{}).Where("id = ?", payer.ID).Update("membership_status", models.MembershipRemoved)
	valid.SessionID = other.ID
	if _, err := rs.SaveGuestRSVP(valid); err == nil {
		t.Fatal("accepted removed payer")
	}
}

func TestConcurrentGuestAndMemberCannotClaimSamePlace(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	payer := newUser(t, "payer")
	session := newSession(t, ss, payer.ID, 1)
	for _, player := range newUsers(t, 5) {
		rsvpIn(t, rs, session.ID, player.ID)
	}
	member := newUser(t, "returning")
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	var guestErr, memberErr error
	go func() {
		defer wg.Done()
		<-start
		_, guestErr = rs.SaveGuestRSVP(GuestRSVPInput{SessionID: session.ID, GuestName: "Alex", ChargedToUserID: payer.ID, Status: models.RSVPStatusIn})
	}()
	go func() {
		defer wg.Done()
		<-start
		_, memberErr = rs.CreateOrUpdateRSVP(RSVPInput{SessionID: session.ID, UserID: member.ID, Status: models.RSVPStatusIn}, false)
	}()
	close(start)
	wg.Wait()
	if memberErr != nil {
		t.Fatal(memberErr)
	}
	if got := summaryOf(t, rs, session.ID); got.TotalIn != 6 {
		t.Fatalf("overbooked: %+v", got)
	}
	memberRSVP := statusOf(t, rs, session.ID, member.ID)
	if (guestErr == nil) != (memberRSVP.Status == models.RSVPStatusWaitlisted) {
		t.Fatalf("unexpected result: guest=%v member=%s", guestErr, memberRSVP.Status)
	}
}

func TestGuestRSVPOutExcludedAndPayerCanBeCorrected(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	payer := newUser(t, "payer")
	nextPayer := newUser(t, "corrected")
	session := newSession(t, ss, payer.ID, 1)
	guest := addGuest(t, rs, session.ID, payer.ID, "Alex")
	in := GuestRSVPInput{ID: guest.ID, SessionID: session.ID, GuestName: "Alex", ChargedToUserID: nextPayer.ID, Status: models.RSVPStatusOut}
	if _, err := rs.SaveGuestRSVP(in); err != nil {
		t.Fatal(err)
	}
	settlement := NewSettlementService(NewLedgerService())
	lines, err := settlement.defaultLines(session.ID, false)
	if err != nil || len(lines) != 0 {
		t.Fatalf("OUT guest was charged: %v %+v", err, lines)
	}
	in.Status = models.RSVPStatusIn
	if _, err := rs.SaveGuestRSVP(in); err != nil {
		t.Fatal(err)
	}
	lines, err = settlement.defaultLines(session.ID, false)
	if err != nil || len(lines) != 1 || lines[0].UserID != nextPayer.ID {
		t.Fatalf("wrong payer: %v %+v", err, lines)
	}
	actor := payer
	actor.Role = models.RoleAdmin
	shuttles := 0
	_, err = NewExpenseService(settlement, NewLedgerService()).input(session.ID, ExpenseInput{TotalHours: 2, ShuttlesUsed: &shuttles}, &actor)
	if err == nil || !strings.Contains(err.Error(), "advanced settlement") {
		t.Fatalf("standard expense omitted guest: %v", err)
	}
	var entries int64
	if err := database.DB.Model(&models.LedgerEntry{}).Count(&entries).Error; err != nil {
		t.Fatal(err)
	}
	if entries != 0 {
		t.Fatal("RSVP change moved money")
	}
	// Guest participants also count on the list of sessions awaiting expenses.
	database.DB.Model(&models.Session{}).Where("id = ?", session.ID).Updates(map[string]any{"ends_at": time.Now().Add(-time.Hour)})
	unsettled, err := NewExpenseService(settlement, NewLedgerService()).ListUnsettledSessions()
	if err != nil || len(unsettled) != 1 || unsettled[0].PlayerCount != 1 {
		t.Fatalf("unsettled count: %v %+v", err, unsettled)
	}
}
