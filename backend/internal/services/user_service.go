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

type CreateUserInput struct {
	Auth0ID        string
	Email          string
	Name           string
	ProfilePicture string
}

// CreateOrUpdateUser creates a new user or updates an existing one
func (s *UserService) CreateOrUpdateUser(input CreateUserInput) (*models.User, bool, error) {
	var user models.User
	isNew := false

	result := database.DB.Where("auth0_id = ?", input.Auth0ID).First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Create new user
			isNew = true
			user = models.User{
				Auth0ID:          input.Auth0ID,
				Email:            input.Email,
				Name:             input.Name,
				ProfilePicture:   input.ProfilePicture,
				Role:             models.RolePending,
				IsPlayer:         true,
				MembershipStatus: models.MembershipPending,
			}

			// Check if this is the admin user
			if s.adminEmail != "" && input.Email == s.adminEmail {
				user.Role = models.RoleAdmin
				user.MembershipStatus = models.MembershipApproved
			}

			if err := database.DB.Create(&user).Error; err != nil {
				return nil, false, err
			}
		} else {
			return nil, false, result.Error
		}
	} else {
		// Update existing user
		user.Name = input.Name
		user.ProfilePicture = input.ProfilePicture
		user.UpdatedAt = time.Now()

		if err := database.DB.Save(&user).Error; err != nil {
			return nil, false, err
		}
	}

	return &user, isNew, nil
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
