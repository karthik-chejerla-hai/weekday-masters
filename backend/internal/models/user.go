package models

import (
	"strings"
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
	// MembershipRemoved is a member an admin has taken out of the club. The row
	// stays: RSVPs, ledger entries and settlements reference it, and the ledger
	// is append-only, so deleting a member would tear a hole in the history that
	// the club-position identity is checked against.
	MembershipRemoved MembershipStatus = "removed"
)

// InvitePlaceholderPrefix marks a user row an admin created before the member
// had ever signed in.
//
// auth0_id is unique and not null, so an invited row cannot simply leave it
// empty — the second invite would collide with the first. Auth0 subjects are
// always "<connection>|<subject>", so a colon-separated prefix can never be
// mistaken for one, and giving each invite its own UUID keeps any number of them
// outstanding at once.
const InvitePlaceholderPrefix = "invite:"

// NewInvitePlaceholder mints an auth0_id for a member who has not signed in yet.
func NewInvitePlaceholder() string {
	return InvitePlaceholderPrefix + uuid.NewString()
}

type User struct {
	ID               uuid.UUID        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Auth0ID          string           `gorm:"size:255;uniqueIndex;not null" json:"auth0_id"`
	Email            string           `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Name             string           `gorm:"size:255;not null" json:"name"`
	Nickname         string           `gorm:"size:100" json:"nickname"`
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

// HasSignedIn reports whether this row is bound to a real Auth0 identity yet.
// An invited member who has never logged in has not, which is what makes their
// email still safe for an admin to correct.
func (u *User) HasSignedIn() bool {
	return !strings.HasPrefix(u.Auth0ID, InvitePlaceholderPrefix)
}

// DisplayName is what the club calls this member: their nickname when they have
// one, otherwise the name Google gave us.
func (u *User) DisplayName() string {
	if u.Nickname != "" {
		return u.Nickname
	}
	return u.Name
}
