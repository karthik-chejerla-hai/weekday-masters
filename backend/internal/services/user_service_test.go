package services

import (
	"testing"

	"github.com/weekday-masters/backend/internal/models"
)

func TestUserService_RegisterUser_AdminBootstrap(t *testing.T) {
	requireDB(t)

	adminEmail := "admin@weekdaymasters.com"
	us := NewUserService(adminEmail)

	// Regular user registration
	regularProfile := &Auth0Profile{
		Sub:           "auth0|regular-123",
		Email:         "regular@example.com",
		Name:          "Regular Player",
		Picture:       "https://example.com/pic.jpg",
		EmailVerified: true,
	}
	regularUser, isNew, err := us.RegisterUser(regularProfile)
	if err != nil {
		t.Fatalf("failed to register regular user: %v", err)
	}
	if !isNew {
		t.Fatal("a first-time registration should report a new user")
	}
	if regularUser.Role != models.RolePending || regularUser.MembershipStatus != models.MembershipPending {
		t.Fatalf("regular user should be pending, got role=%s status=%s", regularUser.Role, regularUser.MembershipStatus)
	}

	// Admin auto-promotion (matching admin email and verified)
	adminProfile := &Auth0Profile{
		Sub:           "auth0|admin-456",
		Email:         adminEmail,
		Name:          "Club Admin",
		Picture:       "https://example.com/admin.jpg",
		EmailVerified: true,
	}
	adminUser, _, err := us.RegisterUser(adminProfile)
	if err != nil {
		t.Fatalf("failed to register admin user: %v", err)
	}
	if adminUser.Role != models.RoleAdmin || adminUser.MembershipStatus != models.MembershipApproved {
		t.Fatalf("admin user should be approved admin, got role=%s status=%s", adminUser.Role, adminUser.MembershipStatus)
	}
}

func TestUserService_MembershipLifecycle(t *testing.T) {
	requireDB(t)
	us := NewUserService("")

	profile := &Auth0Profile{
		Sub:           "auth0|member-789",
		Email:         "member@example.com",
		Name:          "New Member",
		EmailVerified: true,
	}
	user, _, err := us.RegisterUser(profile)
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Check pending list
	pending, err := us.ListPendingJoinRequests()
	if err != nil || len(pending) != 1 {
		t.Fatalf("expected 1 pending join request, got %d (err: %v)", len(pending), err)
	}

	// Approve join request
	approved, err := us.ApproveJoinRequest(user.ID)
	if err != nil {
		t.Fatalf("failed to approve user: %v", err)
	}
	if approved.MembershipStatus != models.MembershipApproved || approved.Role != models.RolePlayer {
		t.Fatalf("expected approved player, got status=%s role=%s", approved.MembershipStatus, approved.Role)
	}

	// Check approved list
	approvedList, err := us.ListApprovedMembers()
	if err != nil || len(approvedList) != 1 {
		t.Fatalf("expected 1 approved member, got %d (err: %v)", len(approvedList), err)
	}

	// Update phone number
	phone := "+61412345678"
	updated, err := us.UpdateProfile(user.ID, UpdateProfileInput{PhoneNumber: &phone})
	if err != nil || updated.PhoneNumber != "+61412345678" {
		t.Fatalf("expected updated phone number, got %s (err: %v)", updated.PhoneNumber, err)
	}

	// Update user role to Admin
	promoted, err := us.UpdateUserRole(user.ID, models.RoleAdmin)
	if err != nil || promoted.Role != models.RoleAdmin {
		t.Fatalf("expected promoted admin role, got %s (err: %v)", promoted.Role, err)
	}
}

func TestUserService_RejectJoinRequest(t *testing.T) {
	requireDB(t)
	us := NewUserService("")

	profile := &Auth0Profile{
		Sub:           "auth0|reject-me",
		Email:         "reject@example.com",
		Name:          "Reject Candidate",
		EmailVerified: true,
	}
	user, _, err := us.RegisterUser(profile)
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	rejected, err := us.RejectJoinRequest(user.ID)
	if err != nil {
		t.Fatalf("failed to reject user: %v", err)
	}
	if rejected.MembershipStatus != models.MembershipRejected {
		t.Fatalf("expected membership status rejected, got %s", rejected.MembershipStatus)
	}

	// Approving an already rejected user should error
	_, err = us.ApproveJoinRequest(user.ID)
	if err == nil {
		t.Fatal("expected error approving non-pending user, got nil")
	}
}
