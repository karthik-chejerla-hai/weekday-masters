// Package seed fills a non-production database with members and sessions that
// cover the states which are tedious to reach by hand — a member below the
// low-balance threshold, a member in debt, an outstanding join request, an
// invite nobody has claimed, and a played session still waiting to be settled.
//
// # Why this is safe to point at a preview
//
// Preview environments are Neon branches of production (see
// .github/workflows/preview-deploy.yml), so a preview database already contains
// every real member, copied. That rules out the usual guard of "refuse to run
// if the database already has users" — it always does. The protection here is
// narrower and stronger instead:
//
// Everything this package writes is tagged, and it never modifies a row it does
// not own. Seeded members are the only ones whose email ends in @seed.invalid
// (a reserved TLD from RFC 2606, so a stray notification can never be delivered
// to a real address), and seeded sessions and transactions carry a "[seed]"
// prefix. A run against the wrong database therefore adds obviously-fake rows
// rather than corrupting real ones — a mess, not a loss.
//
// The interlock that keeps this off production entirely lives in cmd/seed,
// which refuses to start without an explicit environment declaration.
//
// # Re-running
//
// Run is idempotent: it looks for what it would create and leaves it alone if
// it is already there, so the preview workflow can run it on every push to a
// pull request. There is deliberately no reset flag. Undoing the money would
// mean deleting ledger entries, and the ledger is append-only (constitution
// principle VI) — nothing in this codebase deletes an entry, and a seed script
// is a poor place to make the first exception. To start over, recreate the
// database: drop and recreate it locally, or delete the PR's Neon branch.
package seed

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
)

const (
	// EmailDomain marks a row as belonging to the seed. ".invalid" is reserved
	// by RFC 2606 and guaranteed never to resolve, so the notification services
	// cannot deliver anything to a seeded address even if a scheduler fires.
	EmailDomain = "seed.invalid"

	// Tag prefixes the title or description of everything else the seed writes,
	// so a human reading the club's history can tell fixture from fact.
	Tag = "[seed]"
)

// Options controls a run. The zero value is valid.
type Options struct {
	// Now overrides the clock, so tests get deterministic session dates.
	Now time.Time
}

// Report summarises what a run did, for the caller to log. Counts distinguish
// created from reused so a second run visibly does nothing.
type Report struct {
	UsersCreated   int
	UsersReused    int
	SessionsMade   int
	RSVPsMade      int
	MoneyPosted    int
	SettlementMade bool
	Balances       []BalanceLine
}

// BalanceLine is one seeded member's closing balance.
type BalanceLine struct {
	Name        string
	Email       string
	Status      models.MembershipStatus
	BalanceCred int64
}

// memberSpec describes one seeded member and the credit they should start with
// before any session is settled against them.
type memberSpec struct {
	key       string
	name      string
	nickname  string
	role      models.UserRole
	status    models.MembershipStatus
	invited   bool  // no Auth0 identity yet; auth0_id gets the invite: prefix
	topupCent int64 // opening credit, posted as a top-up

	// plays marks the members who appear on the settled session. Their share of
	// it is what carries three of them to their intended end state.
	plays bool
}

// members is the roster. Six people, one per scenario worth testing.
//
// The credit figures are chosen so that after the settled session below charges
// each player roughly $20.33, the club ends up with one member comfortably in
// credit, one just under the $20 low-balance threshold, and one in debt —
// without any of it being posted as an artificial adjustment. They get there by
// playing badminton and not topping up, which is how it happens in real life.
var members = []memberSpec{
	{
		key: "admin", name: "Priya Raman", nickname: "Priya",
		role: models.RoleAdmin, status: models.MembershipApproved,
		topupCent: 12000, plays: true,
	},
	{
		key: "credit", name: "Marcus Webb",
		role: models.RolePlayer, status: models.MembershipApproved,
		topupCent: 8000, plays: true,
	},
	{
		// Ends just below the club's low_balance_threshold_cents (default 2000)
		// while staying positive — the amber chip and the balance_low reminder.
		key: "low", name: "Aiko Tanaka", nickname: "Aiko",
		role: models.RolePlayer, status: models.MembershipApproved,
		topupCent: 4000, plays: true,
	},
	{
		// Never tops up, so the settled session puts them in debt: the red chip,
		// the balance_negative reminder, and a member removal that gets refused.
		key: "negative", name: "Tom Fletcher",
		role: models.RolePlayer, status: models.MembershipApproved,
		topupCent: 0, plays: true,
	},
	{
		// Sits in the /admin join-request queue.
		key: "pending", name: "Sofia Marin",
		role: models.RolePending, status: models.MembershipPending,
	},
	{
		// A real, chargeable row created before a first sign-in. Exercises the
		// invite-adoption path in UserService.RegisterUser.
		key: "invited", name: "Dev Anand",
		role: models.RolePlayer, status: models.MembershipApproved,
		invited: true,
	},
}

// Club assets to open with. Court credit is sized to still cover the next
// session after the settled one draws on it, so the seeded club does not start
// life showing a court_credit_short warning.
const (
	courtCreditCents = 12000
	shuttleCents     = 10000
	shuttleUnits     = 24
)

// Run seeds the database. It is idempotent; see the package comment.
func Run(opts Options) (*Report, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(utils.SydneyLocation)

	// The club row carries the settlement rates, and SeedDefaultClub only fires
	// during migration. Its absence means nobody has migrated this database.
	var clubs int64
	if err := database.DB.Model(&models.Club{}).Count(&clubs).Error; err != nil {
		return nil, fmt.Errorf("checking for the club row: %w", err)
	}
	if clubs == 0 {
		return nil, fmt.Errorf("no club row: run ./cmd/migrate against this database first")
	}

	ledger := services.NewLedgerService()
	report := &Report{}

	users := make(map[string]*models.User, len(members))
	for _, spec := range members {
		user, created, err := ensureUser(spec)
		if err != nil {
			return nil, fmt.Errorf("seeding member %s: %w", spec.key, err)
		}
		users[spec.key] = user
		if created {
			report.UsersCreated++
		} else {
			report.UsersReused++
		}

		// Approval is what earns a ledger account, mirroring UserService.
		if user.IsApproved() {
			if _, err := ledger.EnsurePlayerAccount(user.ID, user.DisplayName()); err != nil {
				return nil, fmt.Errorf("provisioning an account for %s: %w", spec.key, err)
			}
		}
	}

	admin := users["admin"]

	if err := seedAssets(ledger, admin.ID, now, report); err != nil {
		return nil, err
	}
	if err := seedTopups(ledger, admin.ID, now, users, report); err != nil {
		return nil, err
	}

	sessions, err := seedSessions(now, admin.ID, report)
	if err != nil {
		return nil, err
	}
	if err := seedRSVPs(sessions, users, report); err != nil {
		return nil, err
	}
	if err := settlePlayedSession(ledger, sessions["settled"], users, admin.ID, report); err != nil {
		return nil, err
	}

	if err := collectBalances(ledger, users, report); err != nil {
		return nil, err
	}
	return report, nil
}

// email builds the tagged address for a member key.
func email(key string) string { return key + "@" + EmailDomain }

// ensureUser finds the seeded member by their tagged email, or creates them.
//
// Matching on email rather than auth0_id is what lets the invited member keep
// the invite: prefix that makes User.HasSignedIn report false, while still
// being recognisable to a second run.
func ensureUser(spec memberSpec) (*models.User, bool, error) {
	var existing models.User
	err := database.DB.Where("email = ?", email(spec.key)).First(&existing).Error
	if err == nil {
		return &existing, false, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, false, err
	}

	auth0ID := "seed|" + spec.key
	if spec.invited {
		auth0ID = models.NewInvitePlaceholder()
	}

	user := models.User{
		Auth0ID:          auth0ID,
		Email:            email(spec.key),
		Name:             spec.name,
		Nickname:         spec.nickname,
		Role:             spec.role,
		MembershipStatus: spec.status,
		IsPlayer:         true,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		return nil, false, err
	}
	return &user, true, nil
}

// posted reports whether a seeded transaction with this exact description is
// already in the ledger. This is how money stays idempotent: the ledger is
// append-only, so a second run must not post the same movement twice.
func posted(description string) (bool, error) {
	var count int64
	err := database.DB.Model(&models.Transaction{}).
		Where("description = ?", description).Count(&count).Error
	return count > 0, err
}

func seedAssets(ledger *services.LedgerService, by uuid.UUID, now time.Time, report *Report) error {
	court := Tag + " opening court credit"
	if done, err := posted(court); err != nil {
		return err
	} else if !done {
		_, err := ledger.RecordCourtCreditPurchase(services.AssetPurchaseInput{
			AmountCents: courtCreditCents,
			OccurredAt:  now.AddDate(0, 0, -21),
			Description: court,
			CreatedBy:   by,
		})
		if err != nil {
			return fmt.Errorf("seeding court credit: %w", err)
		}
		report.MoneyPosted++
	}

	shuttles := Tag + " opening shuttle stock"
	if done, err := posted(shuttles); err != nil {
		return err
	} else if !done {
		_, err := ledger.RecordShuttlePurchase(services.AssetPurchaseInput{
			AmountCents: shuttleCents,
			Units:       shuttleUnits,
			OccurredAt:  now.AddDate(0, 0, -21),
			Description: shuttles,
			CreatedBy:   by,
		})
		if err != nil {
			return fmt.Errorf("seeding shuttle stock: %w", err)
		}
		report.MoneyPosted++
	}
	return nil
}

func seedTopups(
	ledger *services.LedgerService,
	by uuid.UUID,
	now time.Time,
	users map[string]*models.User,
	report *Report,
) error {
	for _, spec := range members {
		if spec.topupCent == 0 {
			continue
		}
		description := fmt.Sprintf("%s opening credit for %s", Tag, spec.key)
		if done, err := posted(description); err != nil {
			return err
		} else if done {
			continue
		}

		_, err := ledger.RecordTopup(services.CashInput{
			UserID:      users[spec.key].ID,
			AmountCents: spec.topupCent,
			OccurredAt:  now.AddDate(0, 0, -14),
			Description: description,
			CreatedBy:   by,
		})
		if err != nil {
			return fmt.Errorf("topping up %s: %w", spec.key, err)
		}
		report.MoneyPosted++
	}
	return nil
}

// sessionSpec describes one seeded session relative to today.
type sessionSpec struct {
	key      string
	title    string
	daysFrom int // negative is in the past
}

var sessionSpecs = []sessionSpec{
	// Played and settled: gives /money history and the session breakdown content.
	{key: "settled", title: Tag + " Thursday night (settled)", daysFrom: -8},
	// Played but not settled: what the home screen's Settle button acts on.
	{key: "unsettled", title: Tag + " Thursday night (awaiting settlement)", daysFrom: -1},
	// Still to come: RSVPs are open.
	{key: "upcoming", title: Tag + " Thursday night", daysFrom: 5},
}

// seedSessions creates the three sessions directly through GORM rather than
// SessionService, because two of them are in the past and the service refuses
// to create those. The model hooks still derive max_players from the court
// count and resolve starts_at/ends_at in Sydney, so nothing is skipped.
func seedSessions(now time.Time, by uuid.UUID, report *Report) (map[string]*models.Session, error) {
	out := make(map[string]*models.Session, len(sessionSpecs))

	for _, spec := range sessionSpecs {
		var existing models.Session
		err := database.DB.Where("title = ?", spec.title).First(&existing).Error
		if err == nil {
			out[spec.key] = &existing
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}

		date := now.AddDate(0, 0, spec.daysFrom)
		date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, utils.SydneyLocation)

		session := models.Session{
			Title:        spec.title,
			Description:  "Seeded fixture. Safe to delete.",
			SessionDate:  date,
			StartTime:    "19:00",
			EndTime:      "21:00",
			Courts:       1,
			RSVPDeadline: date.AddDate(0, 0, -3),
			Status:       models.SessionStatusOpen,
			CreatedBy:    by,
		}
		if err := database.DB.Create(&session).Error; err != nil {
			return nil, fmt.Errorf("seeding session %s: %w", spec.key, err)
		}
		out[spec.key] = &session
		report.SessionsMade++
	}
	return out, nil
}

// rsvpPlan is who says what to which session. The upcoming session carries a
// mix so the RSVP list has something to render.
//
// Note there is no waitlisted entry: the smallest bookable session is one court
// for six players, and there are only five approved members here, so a waitlist
// cannot be reached honestly at this roster size. Adding two more members would
// unlock it.
var rsvpPlan = map[string]map[string]models.RSVPStatus{
	"settled": {
		"admin": models.RSVPStatusIn, "credit": models.RSVPStatusIn,
		"low": models.RSVPStatusIn, "negative": models.RSVPStatusIn,
	},
	"unsettled": {
		"admin": models.RSVPStatusIn, "credit": models.RSVPStatusIn,
		"low": models.RSVPStatusIn, "negative": models.RSVPStatusIn,
	},
	"upcoming": {
		"admin": models.RSVPStatusIn, "credit": models.RSVPStatusIn,
		"low": models.RSVPStatusIn, "negative": models.RSVPStatusOut,
		"invited": models.RSVPStatusMaybe,
	},
}

// seedRSVPs writes RSVP rows directly. RSVPService is bypassed deliberately: it
// enforces the three-day deadline, which every past session is well beyond.
func seedRSVPs(
	sessions map[string]*models.Session,
	users map[string]*models.User,
	report *Report,
) error {
	for sessionKey, plan := range rsvpPlan {
		session := sessions[sessionKey]
		for userKey, status := range plan {
			user := users[userKey]

			var count int64
			err := database.DB.Model(&models.RSVP{}).
				Where("session_id = ? AND user_id = ?", session.ID, user.ID).
				Count(&count).Error
			if err != nil {
				return err
			}
			if count > 0 {
				continue
			}

			rsvp := models.RSVP{
				SessionID:     session.ID,
				UserID:        user.ID,
				Status:        status,
				RSVPTimestamp: session.RSVPDeadline.Add(-24 * time.Hour),
			}
			if err := database.DB.Create(&rsvp).Error; err != nil {
				return fmt.Errorf("seeding RSVP %s/%s: %w", sessionKey, userKey, err)
			}
			report.RSVPsMade++
		}
	}
	return nil
}

// settlePlayedSession settles the older past session through SettlementService,
// so the charges, the stock drawdown and the club-position identity are all
// produced by the real code path rather than hand-written rows.
//
// The admin brings a guest, which exercises the guest-charged-to-host line.
func settlePlayedSession(
	ledger *services.LedgerService,
	session *models.Session,
	users map[string]*models.User,
	by uuid.UUID,
	report *Report,
) error {
	settlement := services.NewSettlementService(ledger)

	live, err := settlement.LiveSettlementForSession(session.ID)
	if err != nil {
		return fmt.Errorf("checking for an existing settlement: %w", err)
	}
	if live != nil {
		return nil
	}

	lines := make([]services.LineInput, 0, 5)
	for _, spec := range members {
		if !spec.plays {
			continue
		}
		lines = append(lines, services.LineInput{
			UserID: users[spec.key].ID,
			InBase: true,
		})
	}
	lines = append(lines, services.LineInput{
		UserID:    users["admin"].ID,
		GuestName: "Sanjay (guest)",
		InBase:    true,
	})

	if _, _, err := settlement.Settle(services.SettleInput{
		SessionID: session.ID,
		Lines:     lines,
		SettledBy: by,
	}); err != nil {
		return fmt.Errorf("settling the seeded session: %w", err)
	}

	report.SettlementMade = true
	return nil
}

// collectBalances reads back what the seed produced, so the caller can print it
// and a human can see at a glance that the intended states were reached.
func collectBalances(
	ledger *services.LedgerService,
	users map[string]*models.User,
	report *Report,
) error {
	for _, spec := range members {
		user := users[spec.key]
		line := BalanceLine{Name: user.Name, Email: user.Email, Status: user.MembershipStatus}

		// Only approved members have an account to read.
		if user.IsApproved() {
			balance, err := ledger.BalanceOfUser(user.ID)
			if err != nil {
				return fmt.Errorf("reading the balance for %s: %w", spec.key, err)
			}
			line.BalanceCred = balance
		}
		report.Balances = append(report.Balances, line)
	}
	return nil
}
