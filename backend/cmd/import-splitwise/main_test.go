package main

import "testing"

func TestImportTargetRequiresExplicitTargetAndRejectsConnectionOverrides(t *testing.T) {
	for _, tc := range []struct {
		env, dsn, allow, confirm string
		ok                       bool
	}{
		{"local", "postgres://user@127.0.0.1:55439/test?sslmode=disable", "", "", true},
		{"local", "postgres://user@example.test/test", "", "", false},
		{"local", "postgres://user@127.0.0.1/test?host=remote.test", "", "", false},
		{"production", "postgres://user@prod.test/club?sslmode=require", "", "", false},
		{"production", "postgres://user@prod.test/club?sslmode=require", "", "prod.test/club", true},
		{"production", "postgres://user@prod.test:5432/club?sslmode=verify-full&channel_binding=require", "", "prod.test:5432/club", true},
		{"production", "postgres://user@other.test/club?sslmode=require", "", "prod.test/club", false},
		{"production", "postgres://user@prod.test/other?sslmode=require", "", "prod.test/club", false},
		{"production", "postgres://user@prod.test/club", "", "prod.test/club", false},
		{"production", "postgres://user@prod.test/club?sslmode=disable", "", "prod.test/club", false},
		{"production", "postgres://user@prod.test/club?sslmode=require&host=other.test", "", "prod.test/club", false},
		{"production", "postgres://user@prod.test/club?sslmode=require&hostaddr=127.0.0.1", "", "prod.test/club", false},
		{"production", "postgres://user@prod.test/club?sslmode=require&dbname=other", "", "prod.test/club", false},
		{"production", "postgres://user@prod.test/club?sslmode=require&sslmode=disable", "", "prod.test/club", false},
		{"production", "postgres://user@prod.test/?sslmode=require", "", "prod.test/", false},
		{"preview", "postgres://user@preview.test/test", "", "", false},
		{"preview", "postgres://user@preview.test/test", "true", "", true},
	} {
		err := checkTarget(tc.env, tc.dsn, tc.allow, tc.confirm)
		if (err == nil) != tc.ok {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
}
