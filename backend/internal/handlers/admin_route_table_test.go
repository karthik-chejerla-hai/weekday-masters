package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/weekday-masters/backend/internal/models"
)

// Every route under /api/admin must refuse a caller who is not both an approved
// member and an admin. Role and membership status are independent columns, so
// each identity below isolates one half of that conjunction. The paths come
// from the mounted router itself, so a newly registered admin route is covered
// the day it appears, whichever group it was registered on.
func TestAdminRouteTable_RefusesCallersWhoAreNotApprovedAdmins(t *testing.T) {
	h := newHarness(t)

	identities := []struct {
		name string
		user *models.User
	}{
		{"pending admin", makeUser(t, models.RoleAdmin, models.MembershipPending)},
		{"rejected admin", makeUser(t, models.RoleAdmin, models.MembershipRejected)},
		{"removed admin", makeUser(t, models.RoleAdmin, models.MembershipRemoved)},
		{"approved player", makeUser(t, models.RolePlayer, models.MembershipApproved)},
	}

	adminRoutes := 0
	for _, route := range h.router.Routes() {
		if !strings.HasPrefix(route.Path, "/api/admin") {
			continue
		}
		adminRoutes++
		for _, id := range identities {
			res := h.as(id.user).request(route.Method, route.Path, nil)
			if res.Code != http.StatusForbidden {
				t.Errorf("%s %s: expected 403 for %s, got %d: %s",
					route.Method, route.Path, id.name, res.Code, res.Body)
			}
		}
	}
	if adminRoutes == 0 {
		t.Fatal("no /api/admin routes mounted on the harness router")
	}
}

// The conjunction has a passing side too: an approved admin clears both checks.
func TestAdminRouteTable_ApprovedAdminPassesTheMiddleware(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	h.as(admin).get("/api/admin/users").expect(http.StatusOK)
}

// Promotion is the other half of the hole: the role route must not mint an
// admin whose membership is not approved.
func TestUpdateUserRole_RefusesPromotingAnUnapprovedMember(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	pending := makePending(t)

	h.as(admin).put("/api/admin/users/"+pending.ID.String()+"/role",
		map[string]string{"role": "admin"}).expect(http.StatusBadRequest)
}
