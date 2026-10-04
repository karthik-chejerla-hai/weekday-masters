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
	"strings"
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

	// FromCSV seeds the club's real roll from a Splitwise export instead of the
	// six invented members. The file is read at run time and never committed —
	// it carries real names and balances. See roster.go.
	FromCSV string

	// Assets says where the club's money physically sits at the opening
	// balance. Only consulted with FromCSV; nil puts the lot in the bank, which
	// balances but understates nothing except the breakdown.
	Assets *AssetSplit

	// AdminName is the member in the export to make admin. Empty means
	// DefaultAdminName. A club needs at least one admin or nobody can settle a
	// session or approve a join request, so a name that is not in the export is
	// an error rather than a shrug.
	AdminName string
}

// AssetSplit is the club's own side of the opening balance. The ledger records
// the three places money actually sits separately, and derives surplus as the
// balancing figure, so getting this wrong still balances the books while
// misstating the club's position.
type AssetSplit struct {
	BankCents        int64
	CourtCreditCents int64
	ShuttleCents     int64
	ShuttleUnits     int
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

	// Source names the export a roster run came from, empty for a synthetic
	// run. TransactionsRead is how many rows of it reconciled.
	Source           string
	TransactionsRead int
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
//
// With Options.FromCSV set it seeds the club's real roll from an export;
// otherwise it seeds the six invented members that cover the awkward states.
func Run(opts Options) (*Report, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(utils.SydneyLocation)

	if err := requireMigrated(); err != nil {
		return nil, err
	}

	if opts.FromCSV != "" {
		return runFromExport(opts, now)
	}
	return runSynthetic(opts, now)
}

// requireMigrated fails early and legibly on a database nobody has migrated.
// The club row carries the settlement rates, and SeedDefaultClub only fires
// during migration, so its absence is the tell.
func requireMigrated() error {
	var clubs int64
	if err := database.DB.Model(&models.Club{}).Count(&clubs).Error; err != nil {
		return fmt.Errorf("checking for the club row: %w", err)
	}
	if clubs == 0 {
		return fmt.Errorf("no club row: run ./cmd/migrate against this database first")
	}
	return nil
}

func runSynthetic(opts Options, now time.Time) (*Report, error) {
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

	sessions, err := seedSessions(now, admin.ID, report, nil)
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
func seedSessions(
	now time.Time,
	by uuid.UUID,
	report *Report,
	include map[string]bool,
) (map[string]*models.Session, error) {
	out := make(map[string]*models.Session, len(sessionSpecs))

	for _, spec := range sessionSpecs {
		if include != nil && !include[spec.key] {
			continue
		}
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

// --- seeding from a real export -------------------------------------------

// DefaultAdminName is the roster member made admin when Options.AdminName is
// empty. Everyone else is a player. Matched on the export's own spelling.
const DefaultAdminName = "Karthik Chejerla"

// runFromExport seeds the club's real roll and its closing balances.
//
// Two things differ from the synthetic run, both deliberate. The balances are
// posted as one opening-balance transaction rather than a series of top-ups,
// because that is what they are — the closing position of four years kept
// somewhere else, carried over in a single entry. And no session is settled,
// because settling one would charge the players and move them off the figures
// the export states. The point of this fixture is that it matches the
// spreadsheet, so it stays matching it.
func runFromExport(opts Options, now time.Time) (*Report, error) {
	roster, err := LoadRoster(opts.FromCSV)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", opts.FromCSV, err)
	}

	adminName := opts.AdminName
	if adminName == "" {
		adminName = DefaultAdminName
	}
	if err := validateRosterAdmin(roster, adminName); err != nil {
		return nil, err
	}

	openingExists, err := verifyExistingOpeningBalance(roster, opts.Assets)
	if err != nil {
		return nil, err
	}

	ledger := services.NewLedgerService()
	report := &Report{
		Source:           opts.FromCSV,
		TransactionsRead: roster.TransactionCount,
	}

	var admin *models.User
	seeded := make([]*models.User, 0, len(roster.Members))
	openings := make([]services.OpeningPlayerBalance, 0, len(roster.Members))

	for _, member := range roster.Members {
		user, created, err := ensureRosterUser(member, adminName)
		if err != nil {
			return nil, fmt.Errorf("seeding %s: %w", member.Name, err)
		}
		if created {
			report.UsersCreated++
		} else {
			report.UsersReused++
		}

		// Every member on the export has ledger history, including the ones who
		// left at zero, so they all get an account.
		if _, err := ledger.EnsurePlayerAccount(user.ID, user.DisplayName()); err != nil {
			return nil, fmt.Errorf("provisioning an account for %s: %w", member.Name, err)
		}

		openings = append(openings, services.OpeningPlayerBalance{
			UserID:       user.ID,
			BalanceCents: member.BalanceCents,
		})
		if user.MembershipStatus == models.MembershipApproved {
			seeded = append(seeded, user)
		}
		if member.Name == adminName {
			admin = user
		}
	}

	if !openingExists {
		if err := postOpeningBalances(ledger, roster, openings, opts.Assets, admin.ID, now, report); err != nil {
			return nil, err
		}
	}

	// The sessions that move no money: one played and awaiting settlement, one
	// still to come. Settling is left to whoever is testing.
	sessions, err := seedSessions(now, admin.ID, report, map[string]bool{
		"unsettled": true,
		"upcoming":  true,
	})
	if err != nil {
		return nil, err
	}
	if err := seedRosterRSVPs(sessions, seeded, report); err != nil {
		return nil, err
	}

	if err := collectRosterBalances(ledger, roster, report); err != nil {
		return nil, err
	}
	return report, nil
}

// validateRosterAdmin runs before any database writes. A typo must not leave a
// partial roster behind, and a removed member cannot be the club's only admin.
func validateRosterAdmin(roster *Roster, adminName string) error {
	names := make([]string, 0, len(roster.Members))
	for _, member := range roster.Members {
		names = append(names, member.Name)
		if member.Name != adminName {
			continue
		}
		if member.Removed {
			return fmt.Errorf("%q is marked removed and cannot be the seeded admin", adminName)
		}
		return nil
	}
	return fmt.Errorf(
		"the export does not name %q, so there is nobody to make admin; it names: %s",
		adminName, strings.Join(names, ", "))
}

type openingSnapshotValue struct {
	AmountCents int64
	Units       int
	HasUnits    bool
}

func effectiveAssets(roster *Roster, assets *AssetSplit) AssetSplit {
	if assets == nil {
		return AssetSplit{BankCents: roster.MemberCreditCents()}
	}
	return *assets
}

// verifyExistingOpeningBalance makes a repeated import idempotent only when
// the existing transaction has exactly the same semantic entries. A different
// CSV or asset split is an error, since the append-only ledger cannot replace
// the previous opening position.
func verifyExistingOpeningBalance(roster *Roster, assets *AssetSplit) (bool, error) {
	var transactions []models.Transaction
	if err := database.DB.Where("kind = ?", models.TxnOpeningBalance).
		Find(&transactions).Error; err != nil {
		return false, err
	}
	if len(transactions) == 0 {
		return false, nil
	}
	if len(transactions) != 1 {
		return false, fmt.Errorf("found %d opening-balance transactions; expected exactly one", len(transactions))
	}

	type entryRow struct {
		Kind        models.AccountKind
		Email       *string
		AmountCents int64
		Units       *int
	}
	var rows []entryRow
	err := database.DB.Table("ledger_entries AS e").
		Select("a.kind, u.email, e.amount_cents, e.units").
		Joins("JOIN accounts AS a ON a.id = e.account_id").
		Joins("LEFT JOIN users AS u ON u.id = a.user_id").
		Where("e.transaction_id = ?", transactions[0].ID).
		Scan(&rows).Error
	if err != nil {
		return false, err
	}

	actual := make(map[string]openingSnapshotValue, len(rows))
	for _, row := range rows {
		key := string(row.Kind)
		if row.Kind == models.AccountKindPlayer {
			if row.Email == nil {
				return false, fmt.Errorf("opening balance contains a player account without a user")
			}
			key += ":" + *row.Email
		}
		value := actual[key]
		value.AmountCents += row.AmountCents
		if row.Units != nil {
			value.HasUnits = true
			value.Units += *row.Units
		}
		actual[key] = value
	}

	expected := expectedOpeningSnapshot(roster, assets)
	if len(actual) != len(expected) {
		return false, openingBalanceMismatch()
	}
	for key, want := range expected {
		if got, ok := actual[key]; !ok || got != want {
			return false, openingBalanceMismatch()
		}
	}
	return true, nil
}

func expectedOpeningSnapshot(roster *Roster, requested *AssetSplit) map[string]openingSnapshotValue {
	expected := make(map[string]openingSnapshotValue, len(roster.Members)+4)
	for _, member := range roster.Members {
		if member.BalanceCents == 0 {
			continue
		}
		expected[string(models.AccountKindPlayer)+":"+slug(member.Name)+"@"+EmailDomain] =
			openingSnapshotValue{AmountCents: member.BalanceCents}
	}

	assets := effectiveAssets(roster, requested)
	clubAmounts := map[models.AccountKind]int64{
		models.AccountKindBank:         assets.BankCents,
		models.AccountKindCourtCredit:  assets.CourtCreditCents,
		models.AccountKindShuttleStock: assets.ShuttleCents,
	}
	for kind, amount := range clubAmounts {
		if amount == 0 && !(kind == models.AccountKindShuttleStock && assets.ShuttleUnits > 0) {
			continue
		}
		value := openingSnapshotValue{AmountCents: amount}
		if kind == models.AccountKindShuttleStock {
			value.HasUnits = true
			value.Units = assets.ShuttleUnits
		}
		expected[string(kind)] = value
	}
	if surplus := assets.BankCents + assets.CourtCreditCents + assets.ShuttleCents -
		roster.MemberCreditCents(); surplus != 0 {
		expected[string(models.AccountKindSurplus)] = openingSnapshotValue{AmountCents: surplus}
	}
	return expected
}

func openingBalanceMismatch() error {
	return fmt.Errorf("the existing opening balance does not match this export and asset split; recreate the database to import a different opening position")
}

// postOpeningBalances carries the export's closing position into the ledger.
//
// The caller has already verified that no opening-balance transaction exists.
// RecordOpeningBalances performs the same guard inside the write path, closing
// the race with another seed process.
func postOpeningBalances(
	ledger *services.LedgerService,
	roster *Roster,
	openings []services.OpeningPlayerBalance,
	assets *AssetSplit,
	by uuid.UUID,
	now time.Time,
	report *Report,
) error {
	// Nil means "all of it is in the bank". That balances — surplus comes out
	// at zero — but it says the club holds no prepaid court credit and no
	// shuttles, which is a claim about the real world, not an accounting fact.
	effective := effectiveAssets(roster, assets)

	_, err := ledger.RecordOpeningBalances(services.OpeningBalancesInput{
		Players:          openings,
		BankCents:        effective.BankCents,
		CourtCreditCents: effective.CourtCreditCents,
		ShuttleCents:     effective.ShuttleCents,
		ShuttleUnits:     effective.ShuttleUnits,
		OccurredAt:       now,
		CreatedBy:        by,
	})
	if err != nil {
		return fmt.Errorf("posting opening balances: %w", err)
	}
	report.MoneyPosted++
	return nil
}

// ensureRosterUser finds or creates one member of the real roll.
//
// Their name is the export's, but their email is not: it is a @seed.invalid
// address derived from the name, so a fixture member can never be mailed and
// can never be confused for the real person's account.
func ensureRosterUser(member RosterMember, adminName string) (*models.User, bool, error) {
	address := slug(member.Name) + "@" + EmailDomain
	role, status := rosterUserState(member, adminName)

	var existing models.User
	err := database.DB.Where("email = ?", address).First(&existing).Error
	if err == nil {
		if !strings.HasPrefix(existing.Auth0ID, "seed|") {
			return nil, false, fmt.Errorf("fixture address %q belongs to a non-seed user", address)
		}
		updates := map[string]any{
			"name":              member.Name,
			"role":              role,
			"membership_status": status,
			"is_player":         true,
		}
		if err := database.DB.Model(&existing).Updates(updates).Error; err != nil {
			return nil, false, err
		}
		existing.Name = member.Name
		existing.Role = role
		existing.MembershipStatus = status
		existing.IsPlayer = true
		return &existing, false, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, false, err
	}

	user := models.User{
		Auth0ID:          "seed|" + slug(member.Name),
		Email:            address,
		Name:             member.Name,
		Role:             role,
		MembershipStatus: status,
		IsPlayer:         true,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		return nil, false, err
	}
	return &user, true, nil
}

func rosterUserState(member RosterMember, adminName string) (models.UserRole, models.MembershipStatus) {
	role := models.RolePlayer
	if member.Name == adminName {
		role = models.RoleAdmin
	}
	status := models.MembershipApproved
	if member.Removed {
		// Removal is a status change, never a delete: the ledger references the
		// row and is append-only.
		status = models.MembershipRemoved
	}
	return role, status
}

// seedRosterRSVPs puts the current members on both seeded sessions, up to the
// session's capacity so nothing is forced onto a waitlist behind the service's
// back.
func seedRosterRSVPs(
	sessions map[string]*models.Session,
	approved []*models.User,
	report *Report,
) error {
	for _, session := range sessions {
		var confirmed int64
		if err := database.DB.Model(&models.RSVP{}).
			Where("session_id = ? AND status = ?", session.ID, models.RSVPStatusIn).
			Count(&confirmed).Error; err != nil {
			return err
		}

		for _, user := range approved {
			if confirmed >= int64(session.MaxPlayers) {
				break
			}

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
				Status:        models.RSVPStatusIn,
				RSVPTimestamp: session.RSVPDeadline.Add(-24 * time.Hour),
			}
			if err := database.DB.Create(&rsvp).Error; err != nil {
				return fmt.Errorf("seeding RSVP for %s: %w", user.Name, err)
			}
			confirmed++
			report.RSVPsMade++
		}
	}
	return nil
}

// collectRosterBalances reads back each member's balance so the caller can show
// it next to the spreadsheet and see that they agree.
func collectRosterBalances(
	ledger *services.LedgerService,
	roster *Roster,
	report *Report,
) error {
	for _, member := range roster.Members {
		address := slug(member.Name) + "@" + EmailDomain

		var user models.User
		if err := database.DB.Where("email = ?", address).First(&user).Error; err != nil {
			return fmt.Errorf("reading back %s: %w", member.Name, err)
		}

		balance, err := ledger.BalanceOfUser(user.ID)
		if err != nil {
			return fmt.Errorf("reading the balance for %s: %w", member.Name, err)
		}
		report.Balances = append(report.Balances, BalanceLine{
			Name:        user.Name,
			Email:       user.Email,
			Status:      user.MembershipStatus,
			BalanceCred: balance,
		})
	}
	return nil
}
