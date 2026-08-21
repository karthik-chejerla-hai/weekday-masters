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

	if err := database.DB.Create(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
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

// SyncDisplayFields refreshes the cosmetic profile fields on an existing user.
// Email, role and membership status are deliberately not touchable here.
func (s *UserService) SyncDisplayFields(userID uuid.UUID, name, profilePicture string) (*models.User, error) {
	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	if name != "" {
		user.Name = name
	}
	if profilePicture != "" {
		user.ProfilePicture = profilePicture
	}

	if err := database.DB.Save(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (s *UserService) GetUserByID(id uuid.UUID) (*models.User, error) {
	var user models.User
	if err := database.DB.First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByAuth0ID retrieves a user by Auth0 ID
func (s *UserService) GetUserByAuth0ID(auth0ID string) (*models.User, error) {
	var user models.User
	if err := database.DB.First(&user, "auth0_id = ?", auth0ID).Error; err != nil {
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
