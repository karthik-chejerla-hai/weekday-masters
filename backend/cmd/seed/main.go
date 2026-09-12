// Command seed fills a non-production database with test members and sessions.
//
//	go run ./cmd/seed -env=local
//
// The schema must already exist — run ./cmd/migrate first. Re-running is safe;
// see package internal/seed for what it writes and why there is no undo.
//
// # Keeping this away from production
//
// The seed itself is careful (every row it writes is tagged, and it never
// touches a row it does not own), but "careful" is not "cannot". The interlock
// is here, and it is deliberately awkward:
//
//   - -env must be stated explicitly. There is no default, so a bare `go run
//     ./cmd/seed` does nothing but print how to use it.
//   - -env=local refuses any database that is not on the loopback interface,
//     which catches the classic mistake of having a remote DATABASE_URL exported
//     in the shell.
//   - -env=preview additionally requires SEED_ALLOW_PREVIEW=true in the
//     environment. Only .github/workflows/preview-deploy.yml sets it, and it is
//     set inline on the step rather than at workflow level so it cannot leak
//     into a neighbouring job.
//
// There is no -env=production, and deploy.yml does not invoke this binary. If
// production ever genuinely needs fixtures, that should be a considered change
// to this file rather than a flag someone can reach for at 11pm.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"

	"github.com/weekday-masters/backend/internal/config"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/seed"
)

const allowPreviewVar = "SEED_ALLOW_PREVIEW"

func main() {
	env := flag.String("env", "", "target environment: local or preview (required)")
	flag.Parse()

	cfg := config.Load()

	if err := checkTarget(*env, cfg.DatabaseURL, os.Getenv(allowPreviewVar)); err != nil {
		log.Fatalf("refusing to seed: %v", err)
	}

	if err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	report, err := seed.Run(seed.Options{})
	if err != nil {
		log.Fatalf("Seeding failed: %v", err)
	}

	printReport(*env, report)
}

// checkTarget is the whole safety story, kept as a pure function so the tests
// can prove each refusal without a database.
func checkTarget(env, databaseURL, allowPreview string) error {
	switch env {
	case "local":
		local, err := isLoopback(databaseURL)
		if err != nil {
			return err
		}
		if !local {
			return fmt.Errorf(
				"-env=local but DATABASE_URL points at %s, which is not local; "+
					"use -env=preview for a preview branch, and check the DATABASE_URL in your shell",
				hostOf(databaseURL))
		}
		return nil

	case "preview":
		if allowPreview != "true" {
			return fmt.Errorf(
				"-env=preview requires %s=true in the environment; "+
					"it is set by the preview deploy workflow, not by hand", allowPreviewVar)
		}
		return nil

	case "":
		return fmt.Errorf("-env is required (local or preview); there is no default and no production option")

	default:
		return fmt.Errorf("unknown -env %q: expected local or preview", env)
	}
}

// isLoopback reports whether the DSN points at this machine. A hostname that is
// not an IP resolves first, so "localhost" works without hard-coding it and a
// DNS name that happens to point at a remote host is caught.
func isLoopback(databaseURL string) (bool, error) {
	host := hostOf(databaseURL)
	if host == "" {
		return false, fmt.Errorf("DATABASE_URL has no host: %q", redact(databaseURL))
	}

	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback(), nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return false, fmt.Errorf("could not resolve %q to decide whether it is local: %w", host, err)
	}
	if len(ips) == 0 {
		return false, fmt.Errorf("%q resolved to no addresses", host)
	}
	for _, ip := range ips {
		if !ip.IsLoopback() {
			return false, nil
		}
	}
	return true, nil
}

func hostOf(databaseURL string) string {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// redact strips the credentials before a DSN reaches a log line.
func redact(databaseURL string) string {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return "(unparseable)"
	}
	if parsed.User != nil {
		parsed.User = url.UserPassword("***", "***")
	}
	return parsed.String()
}

func printReport(env string, report *seed.Report) {
	log.Printf("Seeded the %s database.", env)
	log.Printf("  members:     %d created, %d already present",
		report.UsersCreated, report.UsersReused)
	log.Printf("  sessions:    %d created", report.SessionsMade)
	log.Printf("  RSVPs:       %d created", report.RSVPsMade)
	log.Printf("  ledger:      %d transactions posted, settlement run: %t",
		report.MoneyPosted, report.SettlementMade)

	log.Println("  closing balances:")
	for _, line := range report.Balances {
		log.Printf("    %-14s %-10s %s",
			line.Name, line.Status, dollars(line.BalanceCred))
	}

	if report.UsersCreated == 0 && report.MoneyPosted == 0 {
		log.Println("Nothing to do — this database was already seeded.")
	}
}

// dollars renders integer cents for a log line. Money never becomes a float
// here any more than it does anywhere else (constitution principle V).
func dollars(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
}
