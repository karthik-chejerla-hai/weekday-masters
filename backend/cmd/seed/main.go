// Command seed loads a Splitwise group export into a local Rally database, so
// the app can be exercised against real balances instead of invented ones.
//
// It is a development tool. It writes users with unroutable @seed.invalid email
// addresses and placeholder Auth0 subjects, so a seeded account can never be
// logged into and can never be emailed. Point it at a scratch database.
//
//	go run ./cmd/seed -splitwise ~/Desktop/Splitwise-current.html
//
// Two things it deliberately does not do:
//
// It will not create the admin's own user row. That row has to come from a real
// Auth0 login — RegisterUser inserts on the Auth0 subject, and a pre-seeded row
// with the same email would collide on the unique index and lock the admin out
// of their own club. Log in once first, then run this.
//
// It will not guess where the club's money is. Splitwise records that the club
// owes its members a total, but not whether that sits in the bank, as prepaid
// court credit, or as shuttles in a bag — which is the entire gap this feature
// exists to close. The split comes from flags, and whatever is left over lands
// in surplus.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/weekday-masters/backend/internal/config"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// clubAccountName is the Splitwise member standing in for the club itself. Its
// balance is the mirror of everyone else's and maps onto Rally's club accounts
// rather than onto a player.
const clubAccountName = "Badminton A."

type member struct {
	Name         string
	BalanceCents int64
}

func main() {
	var (
		path       = flag.String("splitwise", "", "path to the Splitwise group HTML export (required)")
		adminEmail = flag.String("admin-email", "", "the admin's real email; their row must already exist from an Auth0 login")
		adminName  = flag.String("admin-name", "Karthik C.", "how the admin appears in the Splitwise export")
		bankCents  = flag.Int64("bank", 0, "cents held in the club bank account")
		courtCents = flag.Int64("court-credit", 0, "cents of credit prepaid at the venue")
		stockCents = flag.Int64("shuttle-cents", 0, "value in cents of shuttles on hand")
		stockUnits = flag.Int("shuttle-units", 0, "how many shuttles are on hand")
		emailsPath = flag.String("emails", "", "optional file mapping Splitwise names to real email addresses, one \"Name,email\" per line")
		reset      = flag.Bool("reset", false, "delete every ledger entry, transaction and settlement first, so the seed can be run again")
		drop       = flag.String("drop", "", "comma-separated member names to delete entirely before seeding")
		confirm    = flag.Bool("confirm", false, "actually perform -reset and -drop; without it they are only described")
		dryRun     = flag.Bool("dry-run", false, "parse and report without writing anything")
	)
	flag.Parse()

	if *path == "" {
		flag.Usage()
		log.Fatal("seed: -splitwise is required")
	}

	members, err := parseSplitwise(*path)
	if err != nil {
		log.Fatalf("seed: %v", err)
	}

	emails, err := parseEmails(*emailsPath)
	if err != nil {
		log.Fatalf("seed: %v", err)
	}

	var players []member
	var clubOwes int64
	for _, m := range members {
		if m.Name == clubAccountName {
			clubOwes = -m.BalanceCents
			continue
		}
		players = append(players, m)
	}

	var playerTotal int64
	known := make(map[string]bool, len(players))
	for _, p := range players {
		playerTotal += p.BalanceCents
		known[p.Name] = true
	}

	// A misspelt name in the mapping is worse than no mapping: it looks like the
	// balance was handed over when it was not. Refuse rather than warn.
	for name := range emails {
		if !known[name] {
			log.Fatalf("seed: -emails lists %q, which is not a player in the export", name)
		}
	}

	fmt.Printf("Parsed %d players from %s\n\n", len(players), *path)
	for _, p := range players {
		note := "no email — cannot be claimed"
		if addr, ok := emails[p.Name]; ok {
			note = addr
		}
		fmt.Printf("  %-22s %10s  %s\n", p.Name, formatCents(p.BalanceCents), note)
	}
	fmt.Printf("\n  %-22s %s (what the club holds on their behalf)\n", "TOTAL", formatCents(playerTotal))

	if playerTotal != clubOwes {
		log.Fatalf("seed: the export does not balance — players total %s but the club account says %s",
			formatCents(playerTotal), formatCents(clubOwes))
	}
	fmt.Printf("  %-22s balances against the club account exactly\n", "CHECK")

	// Court credit and shuttles are things the admin can look up. Whatever is
	// left is cash, so the bank defaults to the balancing figure rather than
	// dumping the difference into surplus.
	bankDerived := false
	if *bankCents == 0 {
		*bankCents = playerTotal - *courtCents - *stockCents
		bankDerived = true
	}

	assets := *bankCents + *courtCents + *stockCents
	fmt.Printf("\nClub assets: bank %s%s, court credit %s, shuttles %s (%d units)\n",
		formatCents(*bankCents),
		map[bool]string{true: " (derived)", false: ""}[bankDerived],
		formatCents(*courtCents), formatCents(*stockCents), *stockUnits)
	fmt.Printf("Surplus takes the difference: %s\n", formatCents(assets-playerTotal))

	if *dryRun {
		fmt.Println("\nDry run — nothing written.")
		return
	}

	if len(emails) == 0 {
		fmt.Println("\nNo -emails mapping given: these rows get unroutable @seed.invalid addresses")
		fmt.Println("and can never be claimed by a real login. Fine for a scratch database; for a")
		fmt.Println("real club, pass -emails so each balance reaches the person it belongs to.")
	}

	cfg := config.Load()
	if err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatal("seed: failed to connect: ", err)
	}
	// Every statement echoed is noise here; the summary below is the signal.
	database.DB.Logger = gormlogger.Default.LogMode(gormlogger.Silent)
	fmt.Printf("\nSeeding %s\n", redact(cfg.DatabaseURL))

	if *drop != "" || *reset {
		if err := prepare(splitNames(*drop), *reset, *confirm); err != nil {
			log.Fatalf("seed: %v", err)
		}
		if !*confirm {
			fmt.Println("\nNothing written. Re-run with -confirm to carry this out.")
			return
		}
	}

	ledger := services.NewLedgerService()

	// The admin's row must already exist, created by their own Auth0 login.
	//
	// This is fatal rather than a warning. Opening balances are accepted exactly
	// once, so seeding without the admin would strand their balance permanently —
	// the ledger is append-only and there is no second run to catch it.
	var admin models.User
	adminSeeded := false
	if *adminEmail != "" {
		if err := database.DB.Where("email = ?", *adminEmail).First(&admin).Error; err != nil {
			fmt.Printf("\n  The admin (%s) has not logged in yet, so their row does not exist.\n", *adminEmail)
			fmt.Printf("  Seeding now would post opening balances without their %s, and opening\n",
				formatCents(balanceOf(players, *adminName)))
			fmt.Printf("  balances are accepted only once — there would be no second run to fix it.\n\n")
			fmt.Printf("  Start the app, sign in once, then run this again.\n")
			os.Exit(1)
		}
		adminSeeded = true
	}

	opening := make([]services.OpeningPlayerBalance, 0, len(players))
	created, linked := 0, 0

	for _, p := range players {
		if p.Name == *adminName {
			if !adminSeeded {
				continue
			}
			if _, err := ledger.EnsurePlayerAccount(admin.ID, admin.Name); err != nil {
				log.Fatalf("seed: %v", err)
			}
			opening = append(opening, services.OpeningPlayerBalance{UserID: admin.ID, BalanceCents: p.BalanceCents})
			linked++
			continue
		}

		user, isNew, err := ensureSeededUser(p.Name, emails[p.Name])
		if err != nil {
			log.Fatalf("seed: %v", err)
		}
		if isNew {
			created++
		}
		if _, err := ledger.EnsurePlayerAccount(user.ID, user.Name); err != nil {
			log.Fatalf("seed: %v", err)
		}
		opening = append(opening, services.OpeningPlayerBalance{UserID: user.ID, BalanceCents: p.BalanceCents})
	}

	fmt.Printf("  %d players created, %d already existed\n", created, len(opening)-created-linked)
	if adminSeeded {
		fmt.Printf("  admin %q linked to their existing account\n", admin.Name)
	}

	txn, err := ledger.RecordOpeningBalances(services.OpeningBalancesInput{
		Players:          opening,
		BankCents:        *bankCents,
		CourtCreditCents: *courtCents,
		ShuttleUnits:     *stockUnits,
		ShuttleCents:     *stockCents,
		OccurredAt:       utils.NowInSydney(),
		CreatedBy:        adminOrFirst(admin, opening),
	})
	if err != nil {
		if ledgerErr, ok := services.AsLedgerError(err); ok && ledgerErr.Code == services.CodeOpeningBalancesRecorded {
			fmt.Println("\n  Opening balances were already recorded on this database.")
			fmt.Println("  Wipe it and re-migrate to start over — the ledger is append-only by design.")
			os.Exit(1)
		}
		log.Fatalf("seed: failed to record opening balances: %v", err)
	}

	fmt.Printf("  opening balances posted as transaction %s\n", txn.ID)

	position, err := ledger.Position()
	if err != nil {
		log.Fatalf("seed: %v", err)
	}
	fmt.Printf("\nClub position now:\n")
	fmt.Printf("  bank            %s\n", formatCents(position.Assets.BankCents))
	fmt.Printf("  court credit    %s\n", formatCents(position.Assets.CourtCreditCents))
	fmt.Printf("  shuttle stock   %s (%d units)\n", formatCents(position.Assets.ShuttleStockCents), position.Assets.ShuttleStockUnits)
	fmt.Printf("  members prepaid %s\n", formatCents(position.Liabilities.PlayerBalancesCents))
	fmt.Printf("  surplus         %s\n", formatCents(position.SurplusCents))
	fmt.Printf("  balanced        %v\n", position.Balanced)
}

// prepare carries out the destructive options, or describes them and stops.
//
// Both exist for the development phase, where the point is to run the seed,
// find something wrong, and run it again. Opening balances are accepted exactly
// once, so without -reset a second run is refused and the only way back is to
// wipe the database by hand.
//
// Neither is safe on a real club, which is why nothing happens without -confirm:
// on its own each option prints what it would delete and exits.
func prepare(drop []string, reset, confirm bool) error {
	fmt.Printf("\n%s\n", strings.Repeat("-", 60))
	if confirm {
		fmt.Println("DESTRUCTIVE — this will delete the rows below.")
	} else {
		fmt.Println("DESTRUCTIVE — this is what -confirm would delete.")
	}

	// Resolve every name before deleting any of them, so a typo in the second
	// name does not leave the first one already gone.
	targets := make([]models.User, 0, len(drop))
	for _, name := range drop {
		var user models.User
		if err := database.DB.Where("name = ?", name).First(&user).Error; err != nil {
			return fmt.Errorf("-drop names %q, which is not a member on this database", name)
		}
		targets = append(targets, user)
	}

	for _, user := range targets {
		fmt.Printf("  drop member  %-24s %s\n", user.Name, user.Email)
		if confirm {
			if err := dropMember(user); err != nil {
				return fmt.Errorf("dropping %s: %w", user.Name, err)
			}
		}
	}

	if reset {
		for _, t := range ledgerTables {
			var n int64
			if err := database.DB.Table(t).Count(&n).Error; err != nil {
				return err
			}
			fmt.Printf("  reset        %-24s %d rows\n", t, n)
		}
		if confirm {
			if err := resetLedger(); err != nil {
				return fmt.Errorf("resetting the ledger: %w", err)
			}
		}
	}
	fmt.Println(strings.Repeat("-", 60))
	return nil
}

// ledgerTables is every table holding posted money, child-first so the deletes
// do not trip a foreign key.
var ledgerTables = []string{"charge_lines", "settlements", "ledger_entries", "transactions"}

// resetLedger empties the ledger and leaves everything else standing.
//
// Accounts are deliberately kept. Balances are derived by summing entries, never
// stored, so an account with no entries reads as zero — and keeping it means the
// next seed reuses the same account rather than orphaning the old one.
func resetLedger() error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		for _, t := range ledgerTables {
			if err := tx.Exec("DELETE FROM " + t).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// dropMember deletes a member and everything that hangs off them.
//
// Sessions and announcements they created are kept and reassigned to whoever is
// admin: those are club history, and deleting a session because the person who
// scheduled it left would take everyone else's record of it with them.
func dropMember(user models.User) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var admin models.User
		if err := tx.Where("role = ? AND id <> ?", models.RoleAdmin, user.ID).First(&admin).Error; err != nil {
			return fmt.Errorf("no other admin to inherit their sessions: %w", err)
		}

		for _, reassign := range []struct{ table, column string }{
			{"sessions", "created_by"},
			{"announcements", "created_by"},
		} {
			if err := tx.Exec(
				fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s = ?", reassign.table, reassign.column, reassign.column),
				admin.ID, user.ID,
			).Error; err != nil {
				return err
			}
		}

		// Their ledger entries go with their accounts; charge lines name them
		// directly.
		if err := tx.Exec(`DELETE FROM ledger_entries WHERE account_id IN
			(SELECT id FROM accounts WHERE user_id = ?)`, user.ID).Error; err != nil {
			return err
		}
		for _, t := range []string{
			"charge_lines", "accounts", "rsvps", "notifications",
			"user_notification_preferences", "user_push_tokens",
		} {
			if err := tx.Exec("DELETE FROM "+t+" WHERE user_id = ?", user.ID).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&models.User{}, "id = ?", user.ID).Error
	})
}

func splitNames(csv string) []string {
	var names []string
	for _, part := range strings.Split(csv, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			names = append(names, trimmed)
		}
	}
	return names
}

// ensureSeededUser creates an approved player, or returns the existing one.
//
// The Auth0 subject is a placeholder that no real token will ever carry, so a
// seeded account cannot be logged into as it stands. What happens next depends
// on the address:
//
// Given a real one, the row is a balance waiting for its owner —
// UserService.RegisterUser hands it over the first time that person signs in
// with a verified Auth0 email that matches. This is the path for a club moving
// off Splitwise for real.
//
// Without one, the address is unroutable by construction (.invalid is reserved
// by RFC 2606), so the account can neither be emailed nor ever claimed. That is
// the right default for a scratch database and the wrong one for a real club.
func ensureSeededUser(displayName, email string) (*models.User, bool, error) {
	slug := slugify(displayName)
	if email == "" {
		email = slug + "@seed.invalid"
	}

	var existing models.User
	if err := database.DB.Where("email = ?", email).First(&existing).Error; err == nil {
		return &existing, false, nil
	}

	user := models.User{
		Auth0ID:          "seed|" + slug,
		Email:            email,
		Name:             displayName,
		Role:             models.RolePlayer,
		IsPlayer:         true,
		MembershipStatus: models.MembershipApproved,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		return nil, false, fmt.Errorf("creating %q: %w", displayName, err)
	}
	return &user, true, nil
}

// --- parsing --------------------------------------------------------------

var (
	tagPattern     = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	anyTagPattern  = regexp.MustCompile(`<[^>]+>`)
	balancePattern = regexp.MustCompile(`^(gets back|owes|was owed) \$([\d,]+\.\d\d)$`)
)

// parseSplitwise reads the closing balances out of a group export.
//
// It reads the TOTAL BALANCE block rather than replaying the transactions above
// it: those include things Rally has no concept of, like a shared dinner, and
// the closing figure already accounts for them.
func parseSplitwise(path string) ([]member, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	text := tagPattern.ReplaceAllString(string(raw), " ")
	text = anyTagPattern.ReplaceAllString(text, "\n")

	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}

	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "TOTAL BALANCE") {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return nil, fmt.Errorf("no TOTAL BALANCE section in %s — is this a group export?", path)
	}

	var members []member
	for i := start; i+1 < len(lines); i += 2 {
		name, state := lines[i], lines[i+1]

		if state == "settled up" {
			members = append(members, member{Name: name})
			continue
		}

		match := balancePattern.FindStringSubmatch(state)
		if match == nil {
			break // end of the balance block
		}

		cents, err := parseCents(match[2])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if match[1] == "owes" {
			cents = -cents
		}
		members = append(members, member{Name: name, BalanceCents: cents})
	}

	if len(members) == 0 {
		return nil, fmt.Errorf("found the TOTAL BALANCE section but no members under it")
	}
	return members, nil
}

// parseEmails reads a "Splitwise Name,email@example.com" mapping, one per line,
// ignoring blanks and # comments. Names must match the export exactly, because a
// near miss would silently seed an unclaimable row.
func parseEmails(path string) (map[string]string, error) {
	if path == "" {
		return nil, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	mapping := map[string]string{}
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, addr, found := strings.Cut(line, ",")
		name, addr = strings.TrimSpace(name), strings.TrimSpace(addr)
		if !found || name == "" || addr == "" {
			return nil, fmt.Errorf("%s line %d: want \"Name,email\", got %q", path, i+1, line)
		}
		if !strings.Contains(addr, "@") {
			return nil, fmt.Errorf("%s line %d: %q is not an email address", path, i+1, addr)
		}
		if existing, clash := mapping[name]; clash {
			return nil, fmt.Errorf("%s line %d: %q is listed twice (%s and %s)", path, i+1, name, existing, addr)
		}
		mapping[name] = addr
	}
	return mapping, nil
}

// parseCents converts "1,234.56" to 123456 without ever touching a float.
func parseCents(amount string) (int64, error) {
	cleaned := strings.ReplaceAll(amount, ",", "")
	whole, frac, found := strings.Cut(cleaned, ".")
	if !found || len(frac) != 2 {
		return 0, fmt.Errorf("%q is not an amount", amount)
	}

	dollars, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not an amount", amount)
	}
	cents, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not an amount", amount)
	}
	return dollars*100 + cents, nil
}

// --- helpers --------------------------------------------------------------

func slugify(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		// Collapse runs of separators rather than emitting "odd...name".
		case r == ' ' && b.Len() > 0 && !strings.HasSuffix(b.String(), "."):
			b.WriteRune('.')
		}
	}
	return strings.Trim(b.String(), ".")
}

func balanceOf(players []member, name string) int64 {
	for _, p := range players {
		if p.Name == name {
			return p.BalanceCents
		}
	}
	return 0
}

// adminOrFirst attributes the seeding transaction to the admin where they
// exist, and otherwise to whoever came first — it only needs to be somebody.
func adminOrFirst(admin models.User, opening []services.OpeningPlayerBalance) uuid.UUID {
	if admin.ID != uuid.Nil {
		return admin.ID
	}
	if len(opening) > 0 {
		return opening[0].UserID
	}
	return uuid.Nil
}

func formatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
}

// redact hides the password in a connection string before it is printed.
func redact(dsn string) string {
	if at := strings.LastIndex(dsn, "@"); at != -1 {
		if scheme := strings.Index(dsn, "://"); scheme != -1 && scheme+3 < at {
			return dsn[:scheme+3] + "***" + dsn[at:]
		}
	}
	return dsn
}
