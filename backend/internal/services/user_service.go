package services

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
)

// RSVPCanceller withdraws a member from a session. UserService needs it when
// removing a member: a removed player still holding a confirmed spot in an
// upcoming session blocks someone who could actually turn up.
//
// It is an interface rather than a *RSVPService so that removal does not drag
// the notification fan-out into every test that touches membership. The concrete
// implementation is RSVPService, which owns the session row lock and the
// waitlist promotion that freeing a spot triggers.
type RSVPCanceller interface {
	DeleteRSVP(sessionID, userID uuid.UUID, byAdmin bool) error
}

type UserService struct {
	adminEmail string
	rsvps      RSVPCanceller
}

func NewUserService(adminEmail string) *UserService {
	return &UserService{adminEmail: adminEmail}
}

// WithRSVPs supplies the canceller used when removing a member. Without it,
// removal still succeeds but leaves the member's upcoming RSVPs in place.
func (s *UserService) WithRSVPs(rsvps RSVPCanceller) *UserService {
	s.rsvps = rsvps
	return s
}

// NormalizeEmail puts an address into the one form the database stores and
// compares. Google hands back mixed case often enough that matching an invite
// literally would miss.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ErrEmailAlreadyLinked is returned when a second Auth0 identity turns up for an
// email that is already bound to one. Adopting the row would hand one person's
// balance and history to whoever else can prove control of the address.
var ErrEmailAlreadyLinked = errors.New("this email is already linked to a different sign-in")

// ErrInviteEmailNotVerified is returned when someone tries to claim an
// outstanding invite with an unverified email. Invites are addressed by email,
// so an unverified one is not proof of anything.
var ErrInviteEmailNotVerified = errors.New("verify your email address with your provider, then sign in again")

// RegisterUser binds an Auth0 identity to a user row and reports whether it had
// to create one.
//
// Two paths land here. Most people arrive unannounced and become a pending join
// request. Someone an admin invited already has a row — approved, with a role
// and a ledger account waiting — and adopting it is what makes the invite mean
// anything: they sign in and are simply in, rather than queueing for an approval
// that has already been given.
//
// The email MUST come from Auth0 (token claims or /userinfo), never from the request
// body: it decides admin auto-promotion and which invite is claimed, so a
// client-supplied value would let anyone mint an admin account or take over
// someone else's invitation.
func (s *UserService) RegisterUser(profile *Auth0Profile) (*models.User, bool, error) {
	email := NormalizeEmail(profile.Email)

	existing, err := s.FindByEmail(email)
	if err != nil {
		return nil, false, err
	}

	if existing != nil {
		user, err := s.adoptInvite(existing, profile)
		return user, false, err
	}

	user := models.User{
		Auth0ID:          profile.Sub,
		Email:            email,
		Name:             profile.Name,
		ProfilePicture:   profile.Picture,
		Role:             models.RolePending,
		IsPlayer:         true,
		MembershipStatus: models.MembershipPending,
	}

	// Auto-promote the configured admin, but only on a verified email address.
	if s.adminEmail != "" && profile.EmailVerified && email == NormalizeEmail(s.adminEmail) {
		user.Role = models.RoleAdmin
		user.MembershipStatus = models.MembershipApproved
	}

	if err := database.DB.Create(&user).Error; err != nil {
		return nil, false, err
	}

	return &user, true, nil
}

// adoptInvite binds a real Auth0 subject to a row an admin created in advance.
//
// The admin's name and nickname win over Auth0's: they are what the club calls
// this person, and the invite is the more deliberate of the two. Google's
// picture fills in, because the admin never had one to give.
func (s *UserService) adoptInvite(existing *models.User, profile *Auth0Profile) (*models.User, error) {
	if existing.HasSignedIn() {
		return nil, ErrEmailAlreadyLinked
	}
	if !profile.EmailVerified {
		return nil, ErrInviteEmailNotVerified
	}

	existing.Auth0ID = profile.Sub
	if existing.Name == "" {
		existing.Name = profile.Name
	}
	if existing.ProfilePicture == "" {
		existing.ProfilePicture = profile.Picture
	}
	existing.UpdatedAt = time.Now()

	if s.adminEmail != "" && existing.Email == NormalizeEmail(s.adminEmail) {
		existing.Role = models.RoleAdmin
		existing.MembershipStatus = models.MembershipApproved
	}

	if err := database.DB.Save(existing).Error; err != nil {
		return nil, err
	}

	if existing.IsApproved() {
		if _, err := NewLedgerService().EnsurePlayerAccount(existing.ID, existing.DisplayName()); err != nil {
			return nil, err
		}
	}

	return existing, nil
}

// FindByEmail looks a user up by address, case-insensitively, returning
// (nil, nil) when absent. Rows predating email normalization can be stored in
// whatever case Auth0 sent, so the comparison cannot be a plain equality.
func (s *UserService) FindByEmail(email string) (*models.User, error) {
	var user models.User
	err := database.DB.Where("LOWER(email) = ?", NormalizeEmail(email)).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
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
		Order("COALESCE(NULLIF(nickname, ''), name) ASC").
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
	if _, err := NewLedgerService().EnsurePlayerAccount(user.ID, user.DisplayName()); err != nil {
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

// --- admin member management ----------------------------------------------

// ErrMemberNotFound is returned for an id that matches no user row.
var ErrMemberNotFound = errors.New("member not found")

// InviteMemberInput is what an admin types to add someone who has not signed up.
type InviteMemberInput struct {
	Email       string
	Name        string
	Nickname    string
	PhoneNumber string
	Role        models.UserRole
}

// InviteMember adds an approved member ahead of their first sign-in.
//
// Login is Google OAuth, so there is no password to set and no account to create
// on their behalf. What this row does is reserve the identity: it carries the
// membership, the role and the ledger account, and RegisterUser hands all three
// over the moment they sign in with the matching verified email. Until then they
// are a member who simply has not turned up yet — they can be RSVP'd into
// sessions and settled against like anyone else.
func (s *UserService) InviteMember(input InviteMemberInput) (*models.User, error) {
	email := NormalizeEmail(input.Email)
	if err := validateEmail(email); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}

	existing, err := s.FindByEmail(email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("%s is already %s", email, describeMembership(existing))
	}

	role := input.Role
	if role == "" {
		role = models.RolePlayer
	}
	if role != models.RolePlayer && role != models.RoleAdmin {
		return nil, errors.New("invited members must be players or admins")
	}

	user := models.User{
		Auth0ID:          models.NewInvitePlaceholder(),
		Email:            email,
		Name:             name,
		Nickname:         strings.TrimSpace(input.Nickname),
		PhoneNumber:      strings.TrimSpace(input.PhoneNumber),
		Role:             role,
		IsPlayer:         true,
		MembershipStatus: models.MembershipApproved,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		return nil, err
	}

	// Same reason approval creates one: a member without a ledger account cannot
	// be topped up or charged, and an invited member is chargeable immediately.
	if _, err := NewLedgerService().EnsurePlayerAccount(user.ID, user.DisplayName()); err != nil {
		return nil, err
	}

	return &user, nil
}

// UpdateMemberInput carries only the fields an admin may change. A nil pointer
// means "leave alone", which is what lets the form send a partial edit.
type UpdateMemberInput struct {
	Name        *string
	Nickname    *string
	PhoneNumber *string
	Email       *string
	Role        *models.UserRole
	IsPlayer    *bool
}

// UpdateMemberDetails edits a member on an admin's behalf.
//
// Email is editable only while the row is still an unclaimed invite. Once a real
// Auth0 subject is bound to it the address is the identity provider's to state,
// not ours: changing it here would not change how they sign in, and would
// silently move them in or out of the ADMIN_EMAIL auto-promotion rule.
func (s *UserService) UpdateMemberDetails(userID uuid.UUID, input UpdateMemberInput) (*models.User, error) {
	user, err := s.getMember(userID)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, errors.New("name cannot be empty")
		}
		user.Name = name
	}
	if input.Nickname != nil {
		user.Nickname = strings.TrimSpace(*input.Nickname)
	}
	if input.PhoneNumber != nil {
		user.PhoneNumber = strings.TrimSpace(*input.PhoneNumber)
	}
	if input.IsPlayer != nil {
		user.IsPlayer = *input.IsPlayer
	}

	if input.Email != nil {
		email := NormalizeEmail(*input.Email)
		if email != user.Email {
			if user.HasSignedIn() {
				return nil, errors.New("email is managed by the member's sign-in provider and cannot be changed here")
			}
			if err := validateEmail(email); err != nil {
				return nil, err
			}
			clash, err := s.FindByEmail(email)
			if err != nil {
				return nil, err
			}
			if clash != nil {
				return nil, fmt.Errorf("%s is already %s", email, describeMembership(clash))
			}
			user.Email = email
		}
	}

	if input.Role != nil && *input.Role != user.Role {
		if err := s.guardLastAdmin(user, *input.Role); err != nil {
			return nil, err
		}
		user.Role = *input.Role
	}

	user.UpdatedAt = time.Now()
	if err := database.DB.Save(user).Error; err != nil {
		return nil, err
	}

	// The ledger names accounts, and a nickname change that did not reach the
	// balances list would leave two different names for the same person on
	// screen at once.
	if err := syncAccountName(user); err != nil {
		return nil, err
	}

	return user, nil
}

// RemoveMember takes a member out of the club.
//
// The row survives — see models.MembershipRemoved for why — so this revokes
// access rather than deleting a person. Two things have to be true first: the
// club and the member must be square, because a removed member drops off the
// balances list and an unsettled figure there would simply vanish from view; and
// their spots in upcoming sessions have to be given back, because a player who
// is no longer in the club will not be turning up to use them.
func (s *UserService) RemoveMember(userID, actorID uuid.UUID) (*models.User, error) {
	user, err := s.getMember(userID)
	if err != nil {
		return nil, err
	}

	if user.ID == actorID {
		return nil, errors.New("you cannot remove yourself")
	}
	if user.MembershipStatus == models.MembershipRemoved {
		return nil, errors.New("member has already been removed")
	}
	if err := s.guardLastAdmin(user, models.RolePending); err != nil {
		return nil, err
	}

	balance, err := NewLedgerService().BalanceOfUser(user.ID)
	if err != nil {
		return nil, err
	}
	if balance != 0 {
		return nil, fmt.Errorf(
			"%s still has a balance of %s — settle up (record a withdrawal or top-up) before removing them",
			user.DisplayName(), formatCentsForHumans(balance),
		)
	}

	// Free the spots before flipping the status, so a failure here leaves the
	// member in the club rather than half-removed with sessions still held.
	if err := s.cancelUpcomingRSVPs(user.ID); err != nil {
		return nil, err
	}

	user.MembershipStatus = models.MembershipRemoved
	user.Role = models.RolePending
	user.UpdatedAt = time.Now()
	if err := database.DB.Save(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

// ReinstateMember puts a removed member back in the club. Their ledger history
// was never deleted, so they come back to the balance they left with — which,
// removal having required it to be zero, is zero.
func (s *UserService) ReinstateMember(userID uuid.UUID) (*models.User, error) {
	user, err := s.getMember(userID)
	if err != nil {
		return nil, err
	}

	if user.MembershipStatus != models.MembershipRemoved {
		return nil, errors.New("member has not been removed")
	}

	user.MembershipStatus = models.MembershipApproved
	user.Role = models.RolePlayer
	user.UpdatedAt = time.Now()
	if err := database.DB.Save(user).Error; err != nil {
		return nil, err
	}

	if _, err := NewLedgerService().EnsurePlayerAccount(user.ID, user.DisplayName()); err != nil {
		return nil, err
	}

	return user, nil
}

// ListAllMembers returns every user row, newest membership decisions last.
// Unlike ListApprovedMembers this deliberately includes pending, rejected and
// removed rows: the admin screen is where those are acted on.
func (s *UserService) ListAllMembers() ([]models.User, error) {
	var users []models.User
	if err := database.DB.Order("COALESCE(NULLIF(nickname, ''), name) ASC").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// --- helpers ---------------------------------------------------------------

func (s *UserService) getMember(userID uuid.UUID) (*models.User, error) {
	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMemberNotFound
		}
		return nil, err
	}
	return &user, nil
}

// guardLastAdmin refuses a change that would leave the club with no admin. The
// failure mode it prevents is unrecoverable through the UI: with no admin left,
// nobody can promote one.
func (s *UserService) guardLastAdmin(user *models.User, newRole models.UserRole) error {
	if user.Role != models.RoleAdmin || newRole == models.RoleAdmin {
		return nil
	}

	var others int64
	if err := database.DB.Model(&models.User{}).
		Where("role = ? AND membership_status = ? AND id <> ?",
			models.RoleAdmin, models.MembershipApproved, user.ID).
		Count(&others).Error; err != nil {
		return err
	}
	if others == 0 {
		return errors.New("this is the club's only admin — promote someone else first")
	}
	return nil
}

// cancelUpcomingRSVPs withdraws a member from every session still ahead of them,
// promoting whoever was waiting behind them. Sessions already played are left
// alone: they are history, and settlement may already have charged for them.
func (s *UserService) cancelUpcomingRSVPs(userID uuid.UUID) error {
	if s.rsvps == nil {
		return nil
	}

	var sessionIDs []uuid.UUID
	err := database.DB.Model(&models.RSVP{}).
		Joins("JOIN sessions ON sessions.id = rsvps.session_id").
		Where("rsvps.user_id = ? AND sessions.starts_at > ? AND sessions.status <> ?",
			userID, time.Now(), models.SessionStatusCancelled).
		Pluck("rsvps.session_id", &sessionIDs).Error
	if err != nil {
		return err
	}

	for _, sessionID := range sessionIDs {
		if err := s.rsvps.DeleteRSVP(sessionID, userID, true); err != nil {
			return fmt.Errorf("failed to release their spot in an upcoming session: %w", err)
		}
	}
	return nil
}

// syncAccountName keeps the ledger's copy of a member's name in step with the
// name they are displayed under everywhere else.
func syncAccountName(user *models.User) error {
	return database.DB.Model(&models.Account{}).
		Where("user_id = ?", user.ID).
		Update("name", user.DisplayName()).Error
}

func validateEmail(email string) error {
	if email == "" {
		return errors.New("email is required")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return errors.New("that does not look like an email address")
	}
	return nil
}

// describeMembership phrases an existing row for a duplicate-email message, so
// the admin is told whether to approve, reinstate or leave well alone rather
// than just that something is in the way.
func describeMembership(user *models.User) string {
	switch user.MembershipStatus {
	case models.MembershipApproved:
		if user.HasSignedIn() {
			return "a member"
		}
		return "invited and waiting for their first sign-in"
	case models.MembershipPending:
		return "waiting for their join request to be approved"
	case models.MembershipRejected:
		return "a rejected join request"
	case models.MembershipRemoved:
		return "a removed member — reinstate them instead"
	default:
		return "already registered"
	}
}

// formatCentsForHumans renders a balance for an error message. Display is the
// only place cents become dollars.
func formatCentsForHumans(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
}
