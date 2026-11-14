package handlers

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestManualRSVPReminderRoutesRequireApprovedAdmin(t *testing.T) {
	h := newHarness(t)
	path := "/api/admin/sessions/" + uuid.NewString() + "/rsvp-reminder"
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		h.as(nil).request(method, path, nil).expect(http.StatusUnauthorized)
		h.as(makePlayer(t)).request(method, path, nil).expect(http.StatusForbidden)
		h.as(makeAdmin(t)).request(method, path, map[string]any{"request_id": uuid.NewString()}).expect(http.StatusNotFound)
	}
}
