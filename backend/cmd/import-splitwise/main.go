// Command import-splitwise loads reviewed source data through the ledger service.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"

	"github.com/weekday-masters/backend/internal/config"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/services"
	"github.com/weekday-masters/backend/internal/splitwise"
	"gorm.io/gorm/logger"
)

func main() {
	from := flag.String("from", "", "Splitwise CSV path")
	mappingPath := flag.String("mapping", "", "reviewed name-to-email JSON path")
	assetsPath := flag.String("assets", "", "optional verified asset snapshot JSON path")
	apply := flag.Bool("apply", false, "write the reviewed import; default only validates files")
	env := flag.String("env", "local", "local, preview or production")
	confirmTarget := flag.String("confirm-target", "", "production only: exact database host[:port]/database, without credentials")
	allowReversedTopups := flag.Bool("allow-reversed-topups", false, "allow reviewed setup top-ups only when every entry has an exact reversal")
	flag.Parse()
	if *from == "" || *mappingPath == "" {
		log.Fatal("-from and -mapping are required")
	}
	f, err := os.Open(*from)
	check(err)
	defer f.Close()
	source, err := splitwise.Read(f)
	check(err)
	var mapping splitwise.Mapping
	check(readJSON(*mappingPath, &mapping))
	check(source.ValidateMapping(mapping))
	var assets *services.ImportAssets
	if *assetsPath != "" {
		var fields map[string]json.RawMessage
		check(readJSON(*assetsPath, &fields))
		for _, key := range []string{"bank_cents", "court_credit_cents", "shuttle_units", "shuttle_cents"} {
			if raw, ok := fields[key]; !ok || string(raw) == "null" {
				log.Fatalf("asset snapshot requires %s", key)
			}
		}
		assets = &services.ImportAssets{}
		check(readJSON(*assetsPath, assets))
	}
	if !*apply {
		check(json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"valid": true, "source_hash": source.Hash, "transactions": len(source.Rows), "mapped_members": len(mapping.Members), "cutoff": source.Cutoff.Format("2006-01-02"), "writes": false}))
		return
	}
	cfg := config.Load()
	check(checkTarget(*env, cfg.DatabaseURL, os.Getenv("IMPORT_ALLOW_PREVIEW"), *confirmTarget))
	if *env == "production" && (assets == nil || !cfg.NotificationsDisabled) {
		log.Fatal("production import requires -assets and NOTIFICATIONS_DISABLED=true; also disable notifications on the deployed server")
	}
	check(database.Connect(cfg.DatabaseURL))
	database.DB.Logger = logger.Default.LogMode(logger.Silent)
	// Migrations remain an explicit preceding command, never implicit here.
	report, err := services.ImportSplitwiseWithOptions(source, mapping, assets, services.SplitwiseImportOptions{AllowReversedTopups: *allowReversedTopups})
	check(err)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	check(enc.Encode(report))
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
func readJSON(path string, out interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	var extra interface{}
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("unexpected trailing JSON")
	}
	return nil
}
func checkTarget(env, dsn, allowPreview, confirmTarget string) error {
	u, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("invalid database URL")
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return fmt.Errorf("expected PostgreSQL URL")
	}
	if env == "production" {
		if u.Hostname() == "" || u.Path == "" || u.Path == "/" || u.Fragment != "" {
			return fmt.Errorf("production requires an explicit database host and name")
		}
		if confirmTarget == "" || confirmTarget != u.Host+u.Path {
			return fmt.Errorf("production requires -confirm-target matching the database host[:port]/database")
		}
		query, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			return fmt.Errorf("invalid database URL parameters")
		}
		for key, values := range query {
			if (key != "sslmode" && key != "connect_timeout" && key != "channel_binding") || len(values) != 1 {
				return fmt.Errorf("production import rejects connection overrides")
			}
		}
		switch query.Get("sslmode") {
		case "require", "verify-ca", "verify-full":
		default:
			return fmt.Errorf("production import requires an encrypted database connection")
		}
		return nil
	}
	if env == "preview" && allowPreview == "true" {
		return nil
	}
	if env != "local" {
		return fmt.Errorf("expected local, explicitly enabled preview, or confirmed production import")
	}
	host := u.Hostname()
	for key := range u.Query() {
		if key != "sslmode" && key != "connect_timeout" {
			return fmt.Errorf("local import rejects connection overrides")
		}
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("local import requires a loopback database")
	}
	return nil
}
