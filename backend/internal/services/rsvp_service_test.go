package services

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
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
	rs, ss, _ := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)
	users := newUsers(t, 8)

	for _, user := range users {
		rsvpIn(t, rs, session.ID, user.ID)
	}

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

	// Waitlist promotion no longer creates notification history or push work.
	assertNoWaitlistNotifications(t)
}

func TestDeletingConfirmedRSVPPromotesFromWaitlist(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)
	users := newUsers(t, 7)

	for _, user := range users {
		rsvpIn(t, rs, session.ID, user.ID)
	}

	if err := rs.DeleteRSVP(session.ID, users[0].ID, false); err != nil {
		t.Fatalf("failed to delete rsvp: %v", err)
	}

	if got := statusOf(t, rs, session.ID, users[6].ID); got.Status != models.RSVPStatusIn {
		t.Fatalf("waitlisted player should be promoted after a deletion, got %q", got.Status)
	}
	if got := summaryOf(t, rs, session.ID); got.TotalIn != 6 || got.TotalWaitlisted != 0 {
		t.Fatalf("unexpected summary: %+v", got)
	}
	assertNoWaitlistNotifications(t)
}

func TestWaitlistedPlayerWithdrawingPromotesNobody(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)
	users := newUsers(t, 8)

	for _, user := range users {
		rsvpIn(t, rs, session.ID, user.ID)
	}

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
	assertNoWaitlistNotifications(t)
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
	rs, ss, _ := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)
	users := newUsers(t, 9)

	for _, user := range users {
		rsvpIn(t, rs, session.ID, user.ID)
	}
	if got := summaryOf(t, rs, session.ID); got.TotalIn != 6 || got.TotalWaitlisted != 3 {
		t.Fatalf("unexpected setup: %+v", got)
	}

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
	assertNoWaitlistNotifications(t)
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

func assertNoWaitlistNotifications(t *testing.T) {
	t.Helper()
	var count int64
	if err := database.DB.Model(&models.Notification{}).Where("notification_type = ?", models.NotificationWaitlistUpdate).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("waitlist action created %d notification records", count)
	}
}

func TestWaitlistPromotionPreservesExistingNotificationHistory(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)
	users := newUsers(t, 7)
	for _, user := range users {
		rsvpIn(t, rs, session.ID, user.ID)
	}
	historical := models.Notification{UserID: users[6].ID, NotificationType: models.NotificationWaitlistUpdate, Title: "You're in!", Body: "Historical promotion", Data: `{"session_id":"` + session.ID.String() + `"}`}
	if err := database.DB.Create(&historical).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.First(&historical, "id = ?", historical.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := rs.DeleteRSVP(session.ID, users[0].ID, false); err != nil {
		t.Fatal(err)
	}
	if got := statusOf(t, rs, session.ID, users[6].ID); got.Status != models.RSVPStatusIn {
		t.Fatalf("promotion failed: %s", got.Status)
	}
	var stored models.Notification
	if err := database.DB.First(&stored, "id = ?", historical.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Body != historical.Body || stored.Data != historical.Data || stored.Title != historical.Title || stored.NotificationType != historical.NotificationType {
		t.Fatalf("history changed: %+v", stored)
	}
	var count int64
	if err := database.DB.Model(&models.Notification{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected original history only, got %d notifications", count)
	}
}

// deadlinePassed keeps the session upcoming while exercising its stored deadline.
func deadlinePassed(t *testing.T, session *models.Session) {
	t.Helper()
	if err := database.DB.Model(session).UpdateColumn("rsvp_deadline", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
}

func TestLateAttendanceRequests(t *testing.T) {
	for _, initial := range []models.RSVPStatus{"", models.RSVPStatusOut, models.RSVPStatusMaybe} {
		for _, full := range []bool{false, true} {
			t.Run(string(initial)+fmt.Sprint("/full=", full), func(t *testing.T) {
				rs, ss, _ := newTestServices(t)
				session := newSession(t, ss, newUser(t, "organiser").ID, 1)
				user := newUser(t, "late")
				if full {
					for _, u := range newUsers(t, 6) {
						rsvpIn(t, rs, session.ID, u.ID)
					}
				}
				if initial != "" {
					if _, err := rs.CreateOrUpdateRSVP(RSVPInput{session.ID, user.ID, initial}, false); err != nil {
						t.Fatal(err)
					}
				}
				deadlinePassed(t, session)
				got := rsvpIn(t, rs, session.ID, user.ID)
				want := models.RSVPStatusIn
				if full {
					want = models.RSVPStatusWaitlisted
				}
				if got.Status != want || !got.IsLateRSVP || got.AddedByAdmin {
					t.Fatalf("unexpected late response: %+v", got)
				}
				if full && got.WaitlistPosition != 1 {
					t.Fatalf("waitlist position = %d", got.WaitlistPosition)
				}
			})
		}
	}
}

func TestConfirmedPlayerLockedAfterDeadline(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)
	user := newUser(t, "confirmed")
	rsvpIn(t, rs, session.ID, user.ID)
	deadlinePassed(t, session)
	for _, status := range []models.RSVPStatus{models.RSVPStatusOut, models.RSVPStatusMaybe} {
		if _, err := rs.CreateOrUpdateRSVP(RSVPInput{session.ID, user.ID, status}, false); err == nil {
			t.Fatalf("allowed IN to %s", status)
		}
	}
	if err := rs.DeleteRSVP(session.ID, user.ID, false); err == nil {
		t.Fatal("allowed confirmed deletion")
	}
	rsvpIn(t, rs, session.ID, user.ID) // Retrying IN keeps the held spot.
	if got := statusOf(t, rs, session.ID, user.ID); got.Status != models.RSVPStatusIn {
		t.Fatal(got.Status)
	}
	if _, err := rs.CreateOrUpdateRSVP(RSVPInput{session.ID, user.ID, models.RSVPStatusOut}, true); err != nil {
		t.Fatal("admin override:", err)
	}
}

func TestUnconfirmedPlayerMayChangeAfterDeadline(t *testing.T) {
	for _, initial := range []models.RSVPStatus{"", models.RSVPStatusOut, models.RSVPStatusMaybe, models.RSVPStatusWaitlisted} {
		for _, target := range []models.RSVPStatus{models.RSVPStatusOut, models.RSVPStatusMaybe, "delete"} {
			t.Run(string(initial)+"/"+string(target), func(t *testing.T) {
				rs, ss, _ := newTestServices(t)
				session := newSession(t, ss, newUser(t, "organiser").ID, 1)
				user := newUser(t, "unconfirmed")
				if initial == models.RSVPStatusWaitlisted {
					for _, u := range newUsers(t, 6) {
						rsvpIn(t, rs, session.ID, u.ID)
					}
				}
				if initial != "" {
					status := initial
					if initial == models.RSVPStatusWaitlisted {
						status = models.RSVPStatusIn
					}
					if _, err := rs.CreateOrUpdateRSVP(RSVPInput{session.ID, user.ID, status}, false); err != nil {
						t.Fatal(err)
					}
				}
				deadlinePassed(t, session)
				if target == "delete" {
					if initial == "" {
						return
					}
					if err := rs.DeleteRSVP(session.ID, user.ID, false); err != nil {
						t.Fatal(err)
					}
				} else {
					got, err := rs.CreateOrUpdateRSVP(RSVPInput{session.ID, user.ID, target}, false)
					if err != nil {
						t.Fatal(err)
					}
					if got.Status != target {
						t.Fatalf("status = %s", got.Status)
					}
				}
			})
		}
	}
}

func TestLateINClosesAtSessionStart(t *testing.T) {
	for _, offset := range []time.Duration{0, -time.Hour, -24 * time.Hour} {
		t.Run(offset.String(), func(t *testing.T) {
			rs, ss, _ := newTestServices(t)
			session := newSession(t, ss, newUser(t, "organiser").ID, 1)
			user := newUser(t, "late")
			start := time.Now().Add(offset)
			if err := database.DB.Model(session).UpdateColumns(map[string]interface{}{"starts_at": start, "rsvp_deadline": start.Add(-time.Hour)}).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := rs.CreateOrUpdateRSVP(RSVPInput{session.ID, user.ID, models.RSVPStatusIn}, false); err == nil {
				t.Fatal("allowed IN after start")
			}
			got, err := rs.CreateOrUpdateRSVP(RSVPInput{session.ID, user.ID, models.RSVPStatusIn}, true)
			if err != nil || !got.AddedByAdmin {
				t.Fatalf("admin override = %+v, %v", got, err)
			}
		})
	}
}

func TestConfirmedINRetryPreservesOvercapacitySpot(t *testing.T) {
	for _, late := range []bool{false, true} {
		t.Run(fmt.Sprint("late=", late), func(t *testing.T) {
			rs, ss, _ := newTestServices(t)
			session := newSession(t, ss, newUser(t, "organiser").ID, 1)
			users := newUsers(t, 7)
			for _, user := range users[:6] {
				rsvpIn(t, rs, session.ID, user.ID)
			}
			if _, err := rs.CreateOrUpdateRSVP(RSVPInput{session.ID, users[6].ID, models.RSVPStatusIn}, true); err != nil {
				t.Fatal(err)
			}
			if late {
				deadlinePassed(t, session)
			}
			// Both an ordinary confirmed player and the admin-added player retain their spot.
			for _, user := range []models.User{users[0], users[6]} {
				if got := rsvpIn(t, rs, session.ID, user.ID); got.Status != models.RSVPStatusIn {
					t.Fatalf("retry demoted confirmed player: %+v", got)
				}
				if late {
					for _, status := range []models.RSVPStatus{models.RSVPStatusOut, models.RSVPStatusMaybe} {
						if _, err := rs.CreateOrUpdateRSVP(RSVPInput{session.ID, user.ID, status}, false); err == nil {
							t.Fatalf("allowed %s after retry", status)
						}
					}
					if err := rs.DeleteRSVP(session.ID, user.ID, false); err == nil {
						t.Fatal("allowed deletion after retry")
					}
				}
			}
			if got := summaryOf(t, rs, session.ID); got.TotalIn != 7 || got.TotalWaitlisted != 0 {
				t.Fatalf("retry changed attendance: %+v", got)
			}
		})
	}
}

func TestEnteringWaitlistUsesQueueEntryTime(t *testing.T) {
	for _, initial := range []models.RSVPStatus{models.RSVPStatusOut, models.RSVPStatusMaybe} {
		for _, late := range []bool{false, true} {
			t.Run(string(initial)+fmt.Sprint("/late=", late), func(t *testing.T) {
				rs, ss, _ := newTestServices(t)
				session := newSession(t, ss, newUser(t, "organiser").ID, 1)
				confirmed := newUsers(t, 6)
				for _, user := range confirmed {
					rsvpIn(t, rs, session.ID, user.ID)
				}
				oldResponseUser := newUser(t, "old-response")
				old, err := rs.CreateOrUpdateRSVP(RSVPInput{session.ID, oldResponseUser.ID, initial}, false)
				if err != nil {
					t.Fatal(err)
				}
				firstWaitingUser := newUser(t, "first-waiting")
				first := rsvpIn(t, rs, session.ID, firstWaitingUser.ID)
				// Explicit, distinct timestamps make ordering deterministic without sleeps.
				if err := database.DB.Model(old).UpdateColumn("rsvp_timestamp", time.Now().Add(-2*time.Hour)).Error; err != nil {
					t.Fatal(err)
				}
				if err := database.DB.Model(first).UpdateColumn("rsvp_timestamp", time.Now().Add(-time.Hour)).Error; err != nil {
					t.Fatal(err)
				}
				first = statusOf(t, rs, session.ID, firstWaitingUser.ID)
				if late {
					deadlinePassed(t, session)
				}
				joined := rsvpIn(t, rs, session.ID, oldResponseUser.ID)
				if joined.WaitlistPosition != 2 || !joined.RSVPTimestamp.After(first.RSVPTimestamp) {
					t.Fatalf("late join jumped queue: %+v", joined)
				}
				retry := rsvpIn(t, rs, session.ID, firstWaitingUser.ID)
				if retry.WaitlistPosition != 1 || !retry.RSVPTimestamp.Equal(first.RSVPTimestamp) {
					t.Fatalf("retry lost queue position: %+v", retry)
				}
				if _, err := rs.CreateOrUpdateRSVP(RSVPInput{session.ID, confirmed[0].ID, models.RSVPStatusOut}, true); err != nil {
					t.Fatal(err)
				}
				if got := statusOf(t, rs, session.ID, firstWaitingUser.ID); got.Status != models.RSVPStatusIn {
					t.Fatal("did not promote longest-waiting player")
				}
				if got := statusOf(t, rs, session.ID, oldResponseUser.ID); got.Status != models.RSVPStatusWaitlisted {
					t.Fatal("promoted later joiner")
				}
			})
		}
	}
}

func TestINRequiresResolvedSessionStart(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	session := newSession(t, ss, newUser(t, "organiser").ID, 1)
	user := newUser(t, "player")
	if err := database.DB.Model(session).UpdateColumn("starts_at", nil).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := rs.CreateOrUpdateRSVP(RSVPInput{session.ID, user.ID, models.RSVPStatusIn}, false); err == nil {
		t.Fatal("accepted IN without resolved start")
	}
}
