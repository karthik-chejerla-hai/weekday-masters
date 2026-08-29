package services

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
)

type UserService struct {
	adminEmail string
}

func NewUserService(adminEmail string) *UserService {
	return &UserService{adminEmail: adminEmail}
}

// RegisterUser creates a new user from an Auth0-verified profile.
//
// The email MUST come from Auth0 (token claims or /userinfo), never from the request
// body: it decides admin auto-promotion, so a client-supplied value would let anyone
// mint an admin account.
func (s *UserService) RegisterUser(profile *Auth0Profile) (*models.User, error) {
	// A club migrating off Splitwise has rows already carrying balances, created
	// by cmd/seed and waiting for their owner. Adopt one instead of creating a
	// second row for the same person, which would strand the balance.
	if claimed, err := s.claimSeededRow(profile); err != nil {
		return nil, err
	} else if claimed != nil {
		return claimed, nil
	}

	user := models.User{
		Auth0ID:          profile.Sub,
		Email:            profile.Email,
		Name:             profile.Name,
		ProfilePicture:   profile.Picture,
		Role:             models.RolePending,
		IsPlayer:         true,
		MembershipStatus: models.MembershipPending,
	}

	// Auto-promote the configured admin, but only on a verified email address.
	if s.adminEmail != "" && profile.EmailVerified && profile.Email == s.adminEmail {
		user.Role = models.RoleAdmin
		user.MembershipStatus = models.MembershipApproved
	}

	// Emails are unique, so a row already standing on this address means the
	// registration cannot proceed — either the caller's email is unverified and
	// could not claim a seeded row, or that row has already been claimed by
	// someone signing in through a different Auth0 connection. Say so plainly
	// rather than letting the constraint surface as a server error.
	if taken, err := s.emailTaken(profile.Email); err != nil {
		return nil, err
	} else if taken {
		return nil, ErrEmailAlreadyRegistered
	}

	if err := database.DB.Create(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

// ErrEmailAlreadyRegistered reports that a user row already holds the address a
// registration is trying to use.
var ErrEmailAlreadyRegistered = errors.New("a member is already registered with this email address")

func (s *UserService) emailTaken(email string) (bool, error) {
	if email == "" {
		return false, nil
	}
	var n int64
	if err := database.DB.Model(&models.User{}).Where("email = ?", email).Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// SeedAuth0Prefix marks a row created by cmd/seed. It is a placeholder subject
// that no Auth0 token can ever carry, so a seeded row cannot be logged into
// until it is claimed.
const SeedAuth0Prefix = "seed|"

// SeedSubject is the placeholder Auth0 subject cmd/seed gives a row it creates.
// It is derived from the export name, which does not change between runs, so it
// is the stable way to find a row an earlier seed wrote — the email is not, and
// changes the moment a name-to-address mapping is introduced.
func SeedSubject(slug string) string { return SeedAuth0Prefix + slug }

// claimSeededRow hands a seeded row to the person it was created for, returning
// (nil, nil) when there is nothing to claim.
//
// The match is on the email Auth0 vouches for, and only when Auth0 says it is
// verified — the same bar admin auto-promotion is held to, and for the same
// reason: an unverified address is a claim by the caller, not a fact. A row is
// eligible only while its subject still carries the seed prefix, so each row can
// be claimed exactly once.
//
// The UPDATE carries that prefix in its WHERE clause rather than testing it in
// Go, which makes the claim a compare-and-swap: two logins racing for the same
// row leave one of them with RowsAffected == 0.
func (s *UserService) claimSeededRow(profile *Auth0Profile) (*models.User, error) {
	if !profile.EmailVerified || profile.Email == "" {
		return nil, nil
	}

	updates := map[string]any{
		"auth0_id":   profile.Sub,
		"updated_at": time.Now(),
	}
	if profile.Name != "" {
		updates["name"] = profile.Name
	}
	if profile.Picture != "" {
		updates["profile_picture"] = profile.Picture
	}
	// The seeder approves the members it creates, but the admin is defined by
	// ADMIN_EMAIL and has to be promoted on the way in like any other first login.
	if s.adminEmail != "" && profile.Email == s.adminEmail {
		updates["role"] = models.RoleAdmin
		updates["membership_status"] = models.MembershipApproved
	}

	result := database.DB.Model(&models.User{}).
		Where("email = ? AND auth0_id LIKE ?", profile.Email, SeedAuth0Prefix+"%").
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}

	var claimed models.User
	if err := database.DB.Where("auth0_id = ?", profile.Sub).First(&claimed).Error; err != nil {
		return nil, err
	}
	return &claimed, nil
}

// FindByAuth0ID looks up a user by Auth0 subject, returning (nil, nil) when absent.
func (s *UserService) FindByAuth0ID(auth0ID string) (*models.User, error) {
	var user models.User
	err := database.DB.Where("auth0_id = ?", auth0ID).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// SyncDisplayFields refreshes the cosmetic profile fields on an already-loaded user.
// Email, role and membership status are deliberately not touchable here.
func (s *UserService) SyncDisplayFields(user *models.User, name, profilePicture string) (*models.User, error) {
	if name != "" {
		user.Name = name
	}
	if profilePicture != "" {
		user.ProfilePicture = profilePicture
	}
	user.UpdatedAt = time.Now()

	if err := database.DB.Save(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

// GetUserByID retrieves a user by ID
func (s *UserService) GetUserByID(id uuid.UUID) (*models.User, error) {
	var user models.User
	if err := database.DB.First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateProfile updates user profile (phone number)
func (s *UserService) UpdateProfile(userID uuid.UUID, phoneNumber string) (*models.User, error) {
	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	user.PhoneNumber = phoneNumber
	user.UpdatedAt = time.Now()

	if err := database.DB.Save(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

// ListApprovedMembers returns all approved club members
func (s *UserService) ListApprovedMembers() ([]models.User, error) {
	var users []models.User
	if err := database.DB.Where("membership_status = ?", models.MembershipApproved).
		Order("name ASC").
		Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// ListPendingJoinRequests returns all pending membership requests
func (s *UserService) ListPendingJoinRequests() ([]models.User, error) {
	var users []models.User
	if err := database.DB.Where("membership_status = ?", models.MembershipPending).
		Order("created_at ASC").
		Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// ApproveJoinRequest approves a user's membership request
func (s *UserService) ApproveJoinRequest(userID uuid.UUID) (*models.User, error) {
	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	if user.MembershipStatus != models.MembershipPending {
		return nil, errors.New("user is not pending approval")
	}

	user.MembershipStatus = models.MembershipApproved
	user.Role = models.RolePlayer
	user.UpdatedAt = time.Now()

	if err := database.DB.Save(&user).Error; err != nil {
		return nil, err
	}

	// A member without a ledger account cannot be topped up or charged. Creating
	// it here means that never has to be checked for at the point of use.
	if _, err := NewLedgerService().EnsurePlayerAccount(user.ID, user.Name); err != nil {
		return nil, err
	}

	return &user, nil
}

// RejectJoinRequest rejects a user's membership request
func (s *UserService) RejectJoinRequest(userID uuid.UUID) (*models.User, error) {
	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	if user.MembershipStatus != models.MembershipPending {
		return nil, errors.New("user is not pending approval")
	}

	user.MembershipStatus = models.MembershipRejected
	user.UpdatedAt = time.Now()

	if err := database.DB.Save(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

// UpdateUserRole updates a user's role
func (s *UserService) UpdateUserRole(userID uuid.UUID, role models.UserRole) (*models.User, error) {
	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	user.Role = role
	user.UpdatedAt = time.Now()

	if err := database.DB.Save(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}
