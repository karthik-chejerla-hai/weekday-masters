package handlers

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
)

func TestInvitationRoutesRequireApprovedAdmin(t *testing.T) {
	h := newHarness(t)
	member, err := services.NewUserService("").InviteMember(services.InviteMemberInput{Email: "invite@example.com", Name: "Invite Member"})
	if err != nil {
		t.Fatal(err)
	}
	base := "/api/admin/users/" + member.ID.String() + "/invitation"
	body := map[string]string{"request_id": uuid.NewString(), "expected_email": member.Email, "recipient_email": "arbitrary@example.com"}
	for _, actor := range []*models.User{nil, makeUser(t, models.RolePlayer, models.MembershipApproved), makeUser(t, models.RoleAdmin, models.MembershipPending), makeUser(t, models.RoleAdmin, models.MembershipRemoved)} {
		status := http.StatusForbidden
		if actor == nil {
			status = http.StatusUnauthorized
		}
		h.as(actor).get(base).expect(status)
		h.post(base, body).expect(status)
		h.post(base+"/test", body).expect(status)
	}
	admin := makeUser(t, models.RoleAdmin, models.MembershipApproved)
	var preview services.InvitationPreview
	h.as(admin).get(base).expect(200).decode(&preview)
	if preview.TestRecipient != admin.Email || preview.CanSend || preview.CanTest {
		t.Fatalf("wrong send controls %+v", preview)
	}
	h.post(base, body).expect(409)
	h.post(base+"/test", body).expect(409)
	h.post(base, map[string]string{"request_id": "invalid", "expected_email": member.Email}).expect(400)
	h.post(base, map[string]string{}).expect(400)
	h.get("/api/admin/users/not-a-uuid/invitation").expect(400)
	h.get("/api/admin/users/" + uuid.NewString() + "/invitation").expect(404)
}
