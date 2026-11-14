package handlers

import (
	"net/http"
	"testing"
	"time"

	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

func TestRSVPHTTPAfterDeadline(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			h := newHarness(t)
			player := makePlayer(t)
			session := makeSession(t, makeAdmin(t).ID, 1)
			path := "/api/sessions/" + session.ID.String() + "/rsvp"
			if err := database.DB.Model(session).UpdateColumn("rsvp_deadline", time.Now().Add(-time.Hour)).Error; err != nil {
				t.Fatal(err)
			}
			var got models.RSVP
			h.as(player).request(method, path, map[string]any{"status": "in"}).expect(http.StatusOK).decode(&got)
			if got.Status != models.RSVPStatusIn || !got.IsLateRSVP {
				t.Fatalf("unexpected late RSVP: %+v", got)
			}
			for _, status := range []string{"out", "maybe"} {
				h.request(method, path, map[string]any{"status": status}).expect(http.StatusBadRequest)
			}
			h.del(path).expect(http.StatusBadRequest)
			h.get(path + "/me").expect(http.StatusOK).decode(&got)
			if got.Status != models.RSVPStatusIn {
				t.Fatal("rejected request changed confirmed attendance")
			}
		})
	}
}

func TestRSVPHTTPRejectsINAtSessionStart(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)
	session := makeSession(t, makeAdmin(t).ID, 1)
	start := time.Now()
	if err := database.DB.Model(session).UpdateColumns(map[string]any{"starts_at": start, "rsvp_deadline": start.Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	path := "/api/sessions/" + session.ID.String() + "/rsvp"
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		h.as(player).request(method, path, map[string]any{"status": "in"}).expect(http.StatusBadRequest)
	}
	h.get(path + "/me").expect(http.StatusNotFound)
}
