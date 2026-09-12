package main

import (
	"strings"
	"testing"
)

// The guard is the reason this binary is allowed to exist, so it is tested
// harder than the seeding itself. None of these touch a database.

func TestSeedRefusesWithoutAnEnvironment(t *testing.T) {
	err := checkTarget("", "postgres://u:p@localhost:5432/db", "")
	if err == nil {
		t.Fatal("a bare run must refuse; -env has no default")
	}
}

func TestSeedRefusesAnUnknownEnvironment(t *testing.T) {
	for _, env := range []string{"production", "prod", "staging", "LOCAL"} {
		if err := checkTarget(env, "postgres://u:p@localhost:5432/db", "true"); err == nil {
			t.Errorf("-env=%q was accepted; only local and preview exist", env)
		}
	}
}

func TestLocalAcceptsALoopbackDatabase(t *testing.T) {
	for _, dsn := range []string{
		"postgres://badminton:badminton123@localhost:5432/badminton_club?sslmode=disable",
		"postgres://badminton:badminton123@127.0.0.1:5432/badminton_club?sslmode=disable",
	} {
		if err := checkTarget("local", dsn, ""); err != nil {
			t.Errorf("checkTarget(local, %q) = %v, want accepted", dsn, err)
		}
	}
}

// The mistake this catches is real: a remote DATABASE_URL left exported in the
// shell, and a habitual `-env=local`.
func TestLocalRefusesARemoteDatabase(t *testing.T) {
	dsn := "postgres://u:p@ep-cool-darkness-123456.ap-southeast-2.aws.neon.tech/neondb?sslmode=require"
	err := checkTarget("local", dsn, "")
	if err == nil {
		t.Fatal("-env=local accepted a remote host")
	}
}

func TestPreviewRequiresTheEnvironmentVariable(t *testing.T) {
	dsn := "postgres://u:p@ep-cool-darkness-123456.ap-southeast-2.aws.neon.tech/neondb?sslmode=require"

	if err := checkTarget("preview", dsn, ""); err == nil {
		t.Fatal("-env=preview ran without SEED_ALLOW_PREVIEW; the workflow opt-in is the point")
	}
	if err := checkTarget("preview", dsn, "false"); err == nil {
		t.Fatal("SEED_ALLOW_PREVIEW=false must not count as permission")
	}
	if err := checkTarget("preview", dsn, "true"); err != nil {
		t.Fatalf("checkTarget(preview, opted in) = %v, want accepted", err)
	}
}

func TestRedactHidesCredentials(t *testing.T) {
	got := redact("postgres://badminton:hunter2@db.example.com:5432/club")
	if strings.Contains(got, "hunter2") {
		t.Errorf("redact leaked the password: %q", got)
	}
	if !strings.Contains(got, "db.example.com") {
		t.Errorf("redact should keep the host for diagnosis, got %q", got)
	}
}

func TestDollarsRendersCentsWithoutFloatingPoint(t *testing.T) {
	cases := map[int64]string{
		0: "$0.00", 5: "$0.05", 100: "$1.00", 1967: "$19.67",
		12000: "$120.00", -2033: "-$20.33", -5: "-$0.05",
	}
	for cents, want := range cases {
		if got := dollars(cents); got != want {
			t.Errorf("dollars(%d) = %q, want %q", cents, got, want)
		}
	}
}
