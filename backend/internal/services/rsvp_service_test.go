package services

import (
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/models"
)

func TestOneCourtSessionHoldsSixPlayers(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)

	if session.MaxPlayers != 6 {
		t.Fatalf("1 court should allow 6 players, got %d", session.MaxPlayers)
	}

	for i, user := range newUsers(t, 6) {
		rsvp := rsvpIn(t, rs, session.ID, user.ID)
		if rsvp.Status != models.RSVPStatusIn {
			t.Fatalf("player %d should be confirmed, got %q", i, rsvp.Status)
		}
		if rsvp.WaitlistPosition != 0 {
			t.Fatalf("confirmed player %d should have no waitlist position, got %d", i, rsvp.WaitlistPosition)
		}
	}

	if got := summaryOf(t, rs, session.ID); got.TotalIn != 6 || got.SpotsLeft != 0 || got.TotalWaitlisted != 0 {
		t.Fatalf("unexpected summary: %+v", got)
	}
}

func TestRSVPBeyondCapacityIsWaitlisted(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)
	users := newUsers(t, 9)

	for _, user := range users[:6] {
		rsvpIn(t, rs, session.ID, user.ID)
	}

	// Players 7, 8 and 9 queue up in arrival order.
	for i, user := range users[6:] {
		rsvp := rsvpIn(t, rs, session.ID, user.ID)
		if rsvp.Status != models.RSVPStatusWaitlisted {
			t.Fatalf("overflow player should be waitlisted, got %q", rsvp.Status)
		}
		if want := i + 1; rsvp.WaitlistPosition != want {
			t.Fatalf("expected waitlist position %d, got %d", want, rsvp.WaitlistPosition)
		}
	}

	got := summaryOf(t, rs, session.ID)
	if got.TotalIn != 6 || got.TotalWaitlisted != 3 || got.SpotsLeft != 0 {
		t.Fatalf("unexpected summary: %+v", got)
	}
}

func TestDroppingOutPromotesHeadOfWaitlist(t *testing.T) {
	rs, ss, notifier := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)
	users := newUsers(t, 8)

	for _, user := range users {
		rsvpIn(t, rs, session.ID, user.ID)
	}
	notifier.reset()

	// A confirmed player pulls out.
	if _, err := rs.CreateOrUpdateRSVP(RSVPInput{
		SessionID: session.ID,
		UserID:    users[0].ID,
		Status:    models.RSVPStatusOut,
	}, false); err != nil {
		t.Fatalf("failed to drop out: %v", err)
	}

	// The longest-waiting player takes the spot; the other keeps waiting, moved up.
	if got := statusOf(t, rs, session.ID, users[6].ID); got.Status != models.RSVPStatusIn {
		t.Fatalf("first waitlisted player should be promoted, got %q", got.Status)
	}

	still := statusOf(t, rs, session.ID, users[7].ID)
	if still.Status != models.RSVPStatusWaitlisted {
		t.Fatalf("second waitlisted player should still be waiting, got %q", still.Status)
	}
	if still.WaitlistPosition != 1 {
		t.Fatalf("remaining player should move up to position 1, got %d", still.WaitlistPosition)
	}

	if got := summaryOf(t, rs, session.ID); got.TotalIn != 6 || got.TotalWaitlisted != 1 || got.TotalOut != 1 {
		t.Fatalf("unexpected summary: %+v", got)
	}

	// The promoted player is told about it.
	notified := notifier.notifiedUsers()
	if len(notified) != 1 || notified[0] != users[6].ID {
		t.Fatalf("expected exactly the promoted player to be notified, got %v", notified)
	}
	if got := notifier.sent[0].Type; got != models.NotificationWaitlistUpdate {
		t.Fatalf("expected a waitlist_update notification, got %q", got)
	}
}

func TestDeletingConfirmedRSVPPromotesFromWaitlist(t *testing.T) {
	rs, ss, notifier := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)
	users := newUsers(t, 7)

	for _, user := range users {
		rsvpIn(t, rs, session.ID, user.ID)
	}
	notifier.reset()

	if err := rs.DeleteRSVP(session.ID, users[0].ID, false); err != nil {
		t.Fatalf("failed to delete rsvp: %v", err)
	}

	if got := statusOf(t, rs, session.ID, users[6].ID); got.Status != models.RSVPStatusIn {
		t.Fatalf("waitlisted player should be promoted after a deletion, got %q", got.Status)
	}
	if got := summaryOf(t, rs, session.ID); got.TotalIn != 6 || got.TotalWaitlisted != 0 {
		t.Fatalf("unexpected summary: %+v", got)
	}
	if notified := notifier.notifiedUsers(); len(notified) != 1 || notified[0] != users[6].ID {
		t.Fatalf("expected the promoted player to be notified, got %v", notified)
	}
}

func TestWaitlistedPlayerWithdrawingPromotesNobody(t *testing.T) {
	rs, ss, notifier := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)
	users := newUsers(t, 8)

	for _, user := range users {
		rsvpIn(t, rs, session.ID, user.ID)
	}
	notifier.reset()

	// users[6] is waitlisted; withdrawing frees no confirmed spot.
	if _, err := rs.CreateOrUpdateRSVP(RSVPInput{
		SessionID: session.ID,
		UserID:    users[6].ID,
		Status:    models.RSVPStatusOut,
	}, false); err != nil {
		t.Fatalf("waitlisted player should be able to withdraw: %v", err)
	}

	if got := statusOf(t, rs, session.ID, users[7].ID); got.Status != models.RSVPStatusWaitlisted {
		t.Fatalf("remaining player should still be waitlisted, got %q", got.Status)
	}
	if got := summaryOf(t, rs, session.ID); got.TotalIn != 6 || got.TotalWaitlisted != 1 {
		t.Fatalf("unexpected summary: %+v", got)
	}
	if notified := notifier.notifiedUsers(); len(notified) != 0 {
		t.Fatalf("no promotion should have been notified, got %v", notified)
	}
}

func TestAdminAddBypassesCapacity(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)

	for _, user := range newUsers(t, 6) {
		rsvpIn(t, rs, session.ID, user.ID)
	}

	latecomer := newUser(t, "latecomer")
	rsvp, err := rs.CreateOrUpdateRSVP(RSVPInput{
		SessionID: session.ID,
		UserID:    latecomer.ID,
		Status:    models.RSVPStatusIn,
	}, true)
	if err != nil {
		t.Fatalf("admin add failed: %v", err)
	}

	if rsvp.Status != models.RSVPStatusIn {
		t.Fatalf("admin-added player should be confirmed, got %q", rsvp.Status)
	}
	if !rsvp.AddedByAdmin {
		t.Fatal("admin-added player should be flagged added_by_admin")
	}
	if got := summaryOf(t, rs, session.ID); got.TotalIn != 7 {
		t.Fatalf("admin should be able to exceed the cap, got %+v", got)
	}
}

func TestAddingCourtPromotesEveryoneItCanFit(t *testing.T) {
	rs, ss, notifier := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)
	users := newUsers(t, 9)

	for _, user := range users {
		rsvpIn(t, rs, session.ID, user.ID)
	}
	if got := summaryOf(t, rs, session.ID); got.TotalIn != 6 || got.TotalWaitlisted != 3 {
		t.Fatalf("unexpected setup: %+v", got)
	}
	notifier.reset()

	// A second court raises the cap from 6 to 10, which absorbs the whole waitlist.
	courts := 2
	if _, err := ss.UpdateSession(session.ID, UpdateSessionInput{Courts: &courts}); err != nil {
		t.Fatalf("failed to add a court: %v", err)
	}
	if err := rs.PromoteFromWaitlist(session.ID); err != nil {
		t.Fatalf("promotion failed: %v", err)
	}

	if got := summaryOf(t, rs, session.ID); got.TotalIn != 9 || got.TotalWaitlisted != 0 {
		t.Fatalf("unexpected summary after growth: %+v", got)
	}
	if notified := notifier.notifiedUsers(); len(notified) != 3 {
		t.Fatalf("expected 3 promotion notifications, got %d", len(notified))
	}
}

func TestGrowthPromotesOnlyUpToTheNewCap(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)

	// 6 confirmed + 8 waiting; a second court adds only 4 spots.
	for _, user := range newUsers(t, 14) {
		rsvpIn(t, rs, session.ID, user.ID)
	}

	courts := 2
	if _, err := ss.UpdateSession(session.ID, UpdateSessionInput{Courts: &courts}); err != nil {
		t.Fatalf("failed to add a court: %v", err)
	}
	if err := rs.PromoteFromWaitlist(session.ID); err != nil {
		t.Fatalf("promotion failed: %v", err)
	}

	got := summaryOf(t, rs, session.ID)
	if got.TotalIn != 10 || got.TotalWaitlisted != 4 {
		t.Fatalf("expected 10 in and 4 still waiting, got %+v", got)
	}
}

func TestConcurrentRSVPsCannotOversubscribe(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)
	users := newUsers(t, 20)

	// Everyone RSVPs at once. The session row lock has to serialise the capacity
	// check, otherwise several players read the same free spot and all get in.
	var wg sync.WaitGroup
	errs := make(chan error, len(users))

	for _, user := range users {
		wg.Add(1)
		go func(userID uuid.UUID) {
			defer wg.Done()
			if _, err := rs.CreateOrUpdateRSVP(RSVPInput{
				SessionID: session.ID,
				UserID:    userID,
				Status:    models.RSVPStatusIn,
			}, false); err != nil {
				errs <- err
			}
		}(user.ID)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent rsvp failed: %v", err)
	}

	got := summaryOf(t, rs, session.ID)
	if got.TotalIn != 6 {
		t.Fatalf("expected exactly 6 confirmed under concurrency, got %d", got.TotalIn)
	}
	if got.TotalWaitlisted != 14 {
		t.Fatalf("expected the other 14 to be waitlisted, got %d", got.TotalWaitlisted)
	}
}

func TestWaitlistPositionsAreContiguousInArrivalOrder(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)
	users := newUsers(t, 10)

	for _, user := range users {
		rsvpIn(t, rs, session.ID, user.ID)
	}

	rsvps, err := rs.GetRSVPsForSession(session.ID)
	if err != nil {
		t.Fatalf("failed to list rsvps: %v", err)
	}

	position := 0
	for _, rsvp := range rsvps {
		if rsvp.Status != models.RSVPStatusWaitlisted {
			continue
		}
		position++
		if rsvp.WaitlistPosition != position {
			t.Fatalf("expected contiguous position %d, got %d", position, rsvp.WaitlistPosition)
		}
	}

	if position != 4 {
		t.Fatalf("expected 4 waitlisted players, got %d", position)
	}
}

func TestSwitchingFromWaitlistedToMaybeDoesNotConfirm(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)
	users := newUsers(t, 7)

	for _, user := range users {
		rsvpIn(t, rs, session.ID, user.ID)
	}

	updated, err := rs.CreateOrUpdateRSVP(RSVPInput{
		SessionID: session.ID,
		UserID:    users[6].ID,
		Status:    models.RSVPStatusMaybe,
	}, false)
	if err != nil {
		t.Fatalf("failed to switch to maybe: %v", err)
	}

	if updated.Status != models.RSVPStatusMaybe {
		t.Fatalf("expected maybe, got %q", updated.Status)
	}
	if updated.WaitlistPosition != 0 {
		t.Fatalf("a maybe should carry no waitlist position, got %d", updated.WaitlistPosition)
	}
	if got := summaryOf(t, rs, session.ID); got.TotalIn != 6 || got.TotalWaitlisted != 0 || got.TotalMaybe != 1 {
		t.Fatalf("unexpected summary: %+v", got)
	}
}

func TestRejoiningAFullSessionGoesBackToTheWaitlist(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)
	users := newUsers(t, 7)

	for _, user := range users {
		rsvpIn(t, rs, session.ID, user.ID)
	}

	// A confirmed player drops out — the waiting player is promoted into their spot.
	if _, err := rs.CreateOrUpdateRSVP(RSVPInput{
		SessionID: session.ID,
		UserID:    users[0].ID,
		Status:    models.RSVPStatusOut,
	}, false); err != nil {
		t.Fatalf("failed to drop out: %v", err)
	}

	// Changing their mind now puts them at the back of the queue, not back in.
	rejoined := rsvpIn(t, rs, session.ID, users[0].ID)
	if rejoined.Status != models.RSVPStatusWaitlisted {
		t.Fatalf("expected the returning player to be waitlisted, got %q", rejoined.Status)
	}
	if rejoined.WaitlistPosition != 1 {
		t.Fatalf("expected waitlist position 1, got %d", rejoined.WaitlistPosition)
	}
	if got := summaryOf(t, rs, session.ID); got.TotalIn != 6 || got.TotalWaitlisted != 1 {
		t.Fatalf("unexpected summary: %+v", got)
	}
}
