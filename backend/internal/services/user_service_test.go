package services

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"gorm.io/gorm"

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

// All three entry points must preserve fields owned by other writers, even
// when the request started before a membership or identity change committed.
func TestFoundationF001ProfileWritesReadCurrentFields(t *testing.T) {
	for _, path := range []string{"auth_refresh", "self_profile", "admin_profile"} {
		t.Run(path, func(t *testing.T) {
			requireDB(t)
			us := NewUserService("")
			user := invite(t, us, "current@example.com", "Original Member")
			gate := database.DB.Begin()
			if gate.Error != nil {
				t.Fatal(gate.Error)
			}
			defer gate.Rollback()
			if err := gate.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
				"role": models.RolePending, "membership_status": models.MembershipRemoved,
				"nickname": "Latest", "phone_number": "0400111222",
			}).Error; err != nil {
				t.Fatal(err)
			}
			var pid int
			if err := gate.Raw("SELECT pg_backend_pid()").Scan(&pid).Error; err != nil {
				t.Fatal(err)
			}
			done := make(chan foundationProfileResult, 1)
			finished := foundationTrackWriter(t, func() { gate.Rollback() })
			go func() {
				defer finished()
				var result foundationProfileResult
				switch path {
				case "auth_refresh":
					result.user, result.err = us.SyncDisplayFields(user, "Fresh Member", "fresh-picture")
				case "self_profile":
					phone := "0400333444"
					result.user, result.err = us.UpdateProfile(user.ID, UpdateProfileInput{PhoneNumber: &phone})
				case "admin_profile":
					name := "Fresh Member"
					result.user, result.err = us.UpdateMemberDetails(user.ID, UpdateMemberInput{Name: &name})
				}
				done <- result
			}()
			foundationWaitForBlocker(t, pid)
			if err := gate.Commit().Error; err != nil {
				t.Fatal(err)
			}
			result := foundationProfileDone(t, done)
			if result.err != nil {
				t.Fatal(result.err)
			}
			stored, err := us.GetUserByID(user.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, got := range []*models.User{result.user, stored} {
				if got.Role != models.RolePending || got.MembershipStatus != models.MembershipRemoved || got.Nickname != "Latest" {
					t.Errorf("%s restored stale fields: %+v", path, got)
				}
				if path == "self_profile" {
					if got.PhoneNumber != "0400333444" || got.Name != "Original Member" {
						t.Errorf("unexpected self fields: %+v", got)
					}
				} else {
					if got.Name != "Fresh Member" || got.PhoneNumber != "0400111222" {
						t.Errorf("unexpected profile fields: %+v", got)
					}
				}
				if path == "auth_refresh" && got.ProfilePicture != "fresh-picture" {
					t.Error("picture did not refresh")
				}
			}
			foundationAccountName(t, user.ID, "Latest")
		})
	}
}

func TestFoundationF001StaleAuthRefreshCannotRestoreIdentity(t *testing.T) {
	requireDB(t)
	us := NewUserService("")
	stale := invite(t, us, "old@example.com", "Old Member")
	if err := database.DB.Model(&models.User{}).Where("id = ?", stale.ID).Updates(map[string]interface{}{
		"auth0_id": "google-oauth2|current", "email": "current@example.com", "is_player": false,
		"role": models.RoleAdmin, "membership_status": models.MembershipApproved,
	}).Error; err != nil {
		t.Fatal(err)
	}
	got, err := us.SyncDisplayFields(stale, "Current Name", "picture")
	if err != nil {
		t.Fatal(err)
	}
	if got.Auth0ID != "google-oauth2|current" || got.Email != "current@example.com" || got.IsPlayer || got.Role != models.RoleAdmin {
		t.Fatalf("auth refresh restored stale identity: %+v", got)
	}
	foundationAccountName(t, got.ID, "Current")
	empty, err := us.SyncDisplayFields(stale, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if empty.Name != "Current Name" || empty.ProfilePicture != "picture" {
		t.Fatalf("empty refresh cleared fields: %+v", empty)
	}
}

func TestFoundationF001ProfileAndAccountRollbackTogether(t *testing.T) {
	for _, path := range []string{"auth_refresh", "self_profile", "admin_profile"} {
		t.Run(path, func(t *testing.T) {
			requireDB(t)
			us := NewUserService("")
			user := invite(t, us, "rollback@example.com", "Original Member")
			injected := errors.New("account name write rejected")
			callback := "foundation_f001_account_failure"
			if err := database.DB.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
				if tx.Statement.Table == "accounts" {
					tx.AddError(injected)
				}
			}); err != nil {
				t.Fatal(err)
			}
			defer database.DB.Callback().Update().Remove(callback)
			name, nickname, email := "Changed Member", "Changed", "changed@example.com"
			var err error
			switch path {
			case "auth_refresh":
				_, err = us.SyncDisplayFields(user, name, "new-picture")
			case "self_profile":
				_, err = us.UpdateProfile(user.ID, UpdateProfileInput{Nickname: &nickname})
			case "admin_profile":
				_, err = us.UpdateMemberDetails(user.ID, UpdateMemberInput{Name: &name, Nickname: &nickname, Email: &email})
			}
			if !errors.Is(err, injected) {
				t.Fatalf("expected account failure, got %v", err)
			}
			stored, err := us.GetUserByID(user.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.Name != "Original Member" || stored.Nickname != "" || stored.Email != "rollback@example.com" || stored.ProfilePicture != "" {
				t.Fatalf("user changes escaped rollback: %+v", stored)
			}
			foundationAccountName(t, user.ID, "Original")
		})
	}
}

func TestFoundationF001AdminEmailRechecksClaimedIdentity(t *testing.T) {
	requireDB(t)
	us := NewUserService("")
	user := invite(t, us, "claim@example.com", "Invited Member")
	// This is the winning claim's database write. F002 owns the adoption service
	// refactor; F001 must revalidate against its committed identity either way.
	gate := database.DB.Begin()
	if gate.Error != nil {
		t.Fatal(gate.Error)
	}
	defer gate.Rollback()
	if err := gate.Model(&models.User{}).Where("id = ?", user.ID).Update("auth0_id", "google-oauth2|claimed").Error; err != nil {
		t.Fatal(err)
	}
	var pid int
	if err := gate.Raw("SELECT pg_backend_pid()").Scan(&pid).Error; err != nil {
		t.Fatal(err)
	}
	done := make(chan foundationProfileResult, 1)
	finished := foundationTrackWriter(t, func() { gate.Rollback() })
	go func() {
		defer finished()
		email, nickname := "other@example.com", "New Nickname"
		u, err := us.UpdateMemberDetails(user.ID, UpdateMemberInput{Email: &email, Nickname: &nickname})
		done <- foundationProfileResult{u, err}
	}()
	foundationWaitForBlocker(t, pid)
	if err := gate.Commit().Error; err != nil {
		t.Fatal(err)
	}
	result := foundationProfileDone(t, done)
	if result.err == nil || !strings.Contains(result.err.Error(), "sign-in provider") {
		t.Fatalf("claimed email edit was not refused: %v", result.err)
	}
	stored, err := us.GetUserByID(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Auth0ID != "google-oauth2|claimed" || stored.Email != user.Email || stored.Nickname != "" {
		t.Fatalf("claim overwritten: %+v", stored)
	}
	foundationAccountName(t, user.ID, "Invited")
}

func TestFoundationF001MissingAccountRequiresRepairOnlyForChargeableMembers(t *testing.T) {
	requireDB(t)
	us := NewUserService("")
	user := newUser(t, "missing-account") // deliberately no player account
	nickname := "New Nickname"
	if _, err := us.UpdateProfile(user.ID, UpdateProfileInput{Nickname: &nickname}); err == nil || !strings.Contains(err.Error(), "repair") {
		t.Fatalf("expected actionable missing-account error, got %v", err)
	}
	stored, err := us.GetUserByID(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Nickname != "" {
		t.Fatal("missing-account edit changed the user")
	}
	pending, _, err := us.RegisterUser(&Auth0Profile{Sub: "auth0|pending-profile", Email: "pending@example.com", Name: "Pending Member", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := us.UpdateProfile(pending.ID, UpdateProfileInput{Nickname: &nickname}); err != nil {
		t.Fatal(err)
	}
	refreshed, err := us.SyncDisplayFields(pending, "Fresh Pending", "pending-picture")
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Nickname != nickname || refreshed.Name != "Fresh Pending" {
		t.Fatalf("pending edit lost fields: %+v", refreshed)
	}
	var count int64
	if err := database.DB.Model(&models.Account{}).Where("user_id IN ?", []uuid.UUID{user.ID, pending.ID}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("profile editing silently created accounts")
	}
}

func TestFoundationF001ProfileWaitsForImportBeforeRows(t *testing.T) {
	for _, path := range []string{"auth_refresh", "self_profile", "admin_profile"} {
		t.Run(path, func(t *testing.T) {
			requireDB(t)
			us := NewUserService("")
			user := invite(t, us, "import-gate@example.com", "Original Member")
			gate := database.DB.Begin()
			if gate.Error != nil {
				t.Fatal(gate.Error)
			}
			defer gate.Rollback()
			if err := gate.Exec("LOCK TABLE transactions IN SHARE ROW EXCLUSIVE MODE").Error; err != nil {
				t.Fatal(err)
			}
			var pid int
			if err := gate.Raw("SELECT pg_backend_pid()").Scan(&pid).Error; err != nil {
				t.Fatal(err)
			}
			done := make(chan foundationProfileResult, 1)
			finished := foundationTrackWriter(t, func() { gate.Rollback() })
			go func() {
				defer finished()
				name := "Changed"
				var u *models.User
				var err error
				switch path {
				case "auth_refresh":
					u, err = us.SyncDisplayFields(user, name, "")
				case "self_profile":
					u, err = us.UpdateProfile(user.ID, UpdateProfileInput{Nickname: &name})
				case "admin_profile":
					u, err = us.UpdateMemberDetails(user.ID, UpdateMemberInput{Name: &name})
				}
				done <- foundationProfileResult{u, err}
			}()
			foundationWaitForBlocker(t, pid)
			// Import can still lock these rows while the writer waits at table intent.
			probe := database.DB.Transaction(func(tx *gorm.DB) error {
				if err := tx.Exec("SELECT id FROM users WHERE id = ? FOR UPDATE NOWAIT", user.ID).Error; err != nil {
					return err
				}
				return tx.Exec("SELECT id FROM accounts WHERE user_id = ? FOR UPDATE NOWAIT", user.ID).Error
			})
			if err := gate.Rollback().Error; err != nil {
				t.Fatal(err)
			}
			if result := foundationProfileDone(t, done); result.err != nil {
				t.Fatal(result.err)
			}
			if probe != nil {
				t.Fatalf("profile locked a row before import gate: %v", probe)
			}
		})
	}
}

// Reproduce the real pre-F005 settlement lock schedule, not a replacement SQL
// simulation: Settle owns accounts and pauses immediately before ChargeLine's FK.
func TestFoundationF001EmailAndNicknameDoNotDeadlockSettlement(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	us := NewUserService("")
	user := invite(t, us, "settle-invite@example.com", "Invited Member")
	input := SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID, Lines: []LineInput{{UserID: user.ID, InBase: true}}}
	expected, err := f.settlement.Preview(input)
	if err != nil {
		t.Fatal(err)
	}
	reached := make(chan int, 1)
	release := make(chan struct{})
	var once sync.Once
	resume := func() { once.Do(func() { close(release) }) }
	defer resume()
	callback := "foundation_f001_settlement_charge_barrier"
	if err := database.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table != "charge_lines" {
			return
		}
		var pid int
		if err := tx.Session(&gorm.Session{NewDB: true}).Raw("SELECT pg_backend_pid()").Scan(&pid).Error; err != nil {
			tx.AddError(err)
			return
		}
		reached <- pid
		select {
		case <-release:
		case <-time.After(10 * time.Second):
			tx.AddError(errors.New("settlement barrier timed out"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.DB.Callback().Create().Remove(callback) })
	type settlementResult struct {
		record  *models.Settlement
		preview *SettlementPreview
		err     error
	}
	settled := make(chan settlementResult, 1)
	settlementFinished := foundationTrackWriter(t, resume)
	go func() {
		defer settlementFinished()
		record, preview, err := f.settlement.Settle(input)
		settled <- settlementResult{record, preview, err}
	}()
	var pid int
	select {
	case pid = <-reached:
	case result := <-settled:
		t.Fatalf("settlement missed barrier: %v", result.err)
	case <-time.After(5 * time.Second):
		t.Fatal("settlement did not reach ChargeLine")
	}
	edited := make(chan foundationProfileResult, 1)
	email, nickname := "corrected-invite@example.com", "Ace"
	editorFinished := foundationTrackWriter(t, resume)
	go func() {
		defer editorFinished()
		u, err := us.UpdateMemberDetails(user.ID, UpdateMemberInput{Email: &email, Nickname: &nickname})
		edited <- foundationProfileResult{u, err}
	}()
	foundationWaitForBlocker(t, pid)
	// The user half must not commit while its account-name half is blocked.
	beforeCommit, err := us.GetUserByID(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	atomic := beforeCommit.Email == user.Email && beforeCommit.Nickname == user.Nickname
	// This is exactly the lock ChargeLine's FK will need. If email was already
	// updated, its implicit FOR UPDATE makes this fail even with NO KEY UPDATE first.
	probe := database.DB.Transaction(func(tx *gorm.DB) error {
		return tx.Exec("SELECT id FROM users WHERE id = ? FOR KEY SHARE NOWAIT", user.ID).Error
	})
	resume()
	select {
	case result := <-settled:
		if result.err != nil {
			t.Fatalf("settlement failed: %v", result.err)
		}
		if !reflect.DeepEqual(result.preview.Totals, expected.Totals) || result.preview.StockAfter != expected.StockAfter {
			t.Fatal("profile edit changed settlement costs/stock")
		}
		var line models.ChargeLine
		if err := database.DB.Where("settlement_id = ? AND user_id = ?", result.record.ID, user.ID).First(&line).Error; err != nil {
			t.Fatal(err)
		}
		if line.AmountCents != expected.Lines[0].AmountCents {
			t.Fatal("stored charge differs from reviewed cost")
		}
		balance, err := f.ledger.BalanceOfUser(user.ID)
		if err != nil || balance != -line.AmountCents {
			t.Fatalf("settlement balance=%d, err=%v", balance, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("settlement deadlocked with email/nickname edit")
	}
	result := foundationProfileDone(t, edited)
	if result.err != nil {
		t.Fatal(result.err)
	}
	if probe != nil {
		t.Errorf("editor upgraded user lock before account: %v", probe)
	}
	if !atomic {
		t.Error("identity/name edit committed before account synchronization")
	}
	stored, err := us.GetUserByID(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Email != email || stored.Nickname != nickname || stored.HasSignedIn() {
		t.Fatalf("incorrect saved invite: %+v", stored)
	}
	foundationAccountName(t, user.ID, nickname)
	assertBalanced(t)
}

type foundationProfileResult struct {
	user *models.User
	err  error
}

func foundationProfileDone(t *testing.T, done <-chan foundationProfileResult) foundationProfileResult {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("profile write did not finish within timeout")
		return foundationProfileResult{}
	}
}

func foundationWaitForBlocker(t *testing.T, pid int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		err := database.DB.WithContext(ctx).Raw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE ? = ANY(pg_blocking_pids(pid)))`, pid).Scan(&waiting).Error
		if err != nil {
			t.Fatalf("observe profile lock wait: %v", err)
		}
		if waiting {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("writer never blocked behind database PID %d", pid)
		case <-ticker.C:
		}
	}
}

func foundationAccountName(t *testing.T, userID uuid.UUID, want string) {
	t.Helper()
	var account models.Account
	if err := database.DB.Where("user_id = ?", userID).First(&account).Error; err != nil {
		t.Fatal(err)
	}
	if account.Name != want {
		t.Errorf("account name=%q, want %q", account.Name, want)
	}
}

// Join every writer before the next test can truncate its tables, including
// failure paths. Release database/channel barriers before waiting for completion.
func foundationTrackWriter(t *testing.T, release func()) func() {
	t.Helper()
	finished := make(chan struct{})
	t.Cleanup(func() {
		release()
		select {
		case <-finished:
		case <-time.After(20 * time.Second):
			t.Error("profile/settlement writer did not stop after barrier release")
		}
	})
	return func() { close(finished) }
}

func TestFoundationF001AdminBootstrapHasAccountBeforeNextLogin(t *testing.T) {
	for _, failAccount := range []bool{false, true} {
		label := "success"
		if failAccount {
			label = "account_failure"
		}
		t.Run(label, func(t *testing.T) {
			requireDB(t)
			us := NewUserService("bootstrap@example.com")
			profile := &Auth0Profile{Sub: "google-oauth2|bootstrap", Email: "bootstrap@example.com", Name: "Bootstrap Admin", EmailVerified: true}
			injected := errors.New("bootstrap account creation failed")
			if failAccount {
				callback := "foundation_f001_bootstrap_account_failure"
				if err := database.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
					if tx.Statement.Table == "accounts" {
						tx.AddError(injected)
					}
				}); err != nil {
					t.Fatal(err)
				}
				defer database.DB.Callback().Create().Remove(callback)
			}
			user, isNew, err := us.RegisterUser(profile)
			if failAccount {
				if !errors.Is(err, injected) {
					t.Fatalf("expected account failure, got %v", err)
				}
				var users, accounts int64
				if err := database.DB.Model(&models.User{}).Where("auth0_id = ?", profile.Sub).Count(&users).Error; err != nil {
					t.Fatal(err)
				}
				if err := database.DB.Model(&models.Account{}).Where("kind = ?", models.AccountKindPlayer).Count(&accounts).Error; err != nil {
					t.Fatal(err)
				}
				if users != 0 || accounts != 0 {
					t.Fatalf("bootstrap left partial membership: %d users, %d accounts", users, accounts)
				}
				return
			}
			if err != nil || !isNew || user == nil {
				t.Fatalf("bootstrap failed: isNew=%v, err=%v", isNew, err)
			}
			if !user.IsAdmin() || !user.IsApproved() {
				t.Fatalf("bootstrap promotion changed: %+v", user)
			}
			foundationAccountName(t, user.ID, "Bootstrap")
			refreshed, err := us.SyncDisplayFields(user, "Returning Admin", "picture")
			if err != nil {
				t.Fatalf("next bootstrap login failed: %v", err)
			}
			if !refreshed.IsAdmin() || !refreshed.IsApproved() {
				t.Fatal("next login lost admin membership")
			}
			foundationAccountName(t, user.ID, "Returning")
		})
	}
}
