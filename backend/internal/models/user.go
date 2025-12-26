package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRole string

const (
	RolePending UserRole = "pending"
	RolePlayer  UserRole = "player"
	RoleAdmin   UserRole = "admin"
)

type MembershipStatus string

const (
	MembershipPending  MembershipStatus = "pending"
	MembershipApproved MembershipStatus = "approved"
	MembershipRejected MembershipStatus = "rejected"
)

type User struct {
	ID               uuid.UUID        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Auth0ID          string           `gorm:"size:255;uniqueIndex;not null" json:"auth0_id"`
	Email            string           `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Name             string           `gorm:"size:255;not null" json:"name"`
	ProfilePicture   string           `gorm:"type:text" json:"profile_picture"`
	PhoneNumber      string           `gorm:"size:50" json:"phone_number"`
	Role             UserRole         `gorm:"size:50;not null;default:'pending'" json:"role"`
	IsPlayer         bool             `gorm:"default:true" json:"is_player"`
	MembershipStatus MembershipStatus `gorm:"size:50;default:'pending'" json:"membership_status"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (u *User) IsApproved() bool {
	return u.MembershipStatus == MembershipApproved
}

func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}
