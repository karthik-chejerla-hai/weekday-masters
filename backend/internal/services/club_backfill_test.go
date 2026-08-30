package services

import (
	"testing"

	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

// A club that predates the ledger gets its new columns from AutoMigrate as
// zeros. Nothing else fills them in: SeedDefaultClub only fires when there is no
// club at all. A zero base rate is not a free session — it is a settlement that
// charges nobody anything, and a low-balance warning that never fires.
func TestMigrateBackfillsSettingsOnAnExistingClub(t *testing.T) {
	requireDB(t)

	// Put the database in the state an upgraded club is in.
	if err := database.DB.Model(&models.Club{}).Where("1 = 1").Updates(map[string]any{
		"base_hours":                  0,
		"base_rate_cents":             0,
		"extra_rate_cents":            0,
		"shuttles_per_hour":           0,
		"low_balance_threshold_cents": 0,
	}).Error; err != nil {
		t.Fatalf("clearing settings: %v", err)
	}

	if err := database.Migrate(); err != nil {
		t.Fatalf("migrating: %v", err)
	}

	var club models.Club
	if err := database.DB.First(&club).Error; err != nil {
		t.Fatalf("reloading: %v", err)
	}

	if club.BaseRateCents != 3000 {
		t.Errorf("base rate = %d, want 3000", club.BaseRateCents)
	}
	if club.ExtraRateCents != 2300 {
		t.Errorf("extra rate = %d, want 2300", club.ExtraRateCents)
	}
	if club.ShuttlesPerHour != 5 {
		t.Errorf("shuttles per hour = %v, want 5", club.ShuttlesPerHour)
	}
	if club.BaseHours != 2 {
		t.Errorf("base hours = %v, want 2", club.BaseHours)
	}
	if club.LowBalanceThresholdCents != 2000 {
		t.Errorf("low balance threshold = %d, want 2000", club.LowBalanceThresholdCents)
	}
}

// The columns are defaults for the settlement form, and an admin who has set a
// rate deliberately has set a non-zero one. Re-running migrations must not walk
// over that.
func TestMigrateLeavesConfiguredSettingsAlone(t *testing.T) {
	requireDB(t)

	if err := database.DB.Model(&models.Club{}).Where("1 = 1").Updates(map[string]any{
		"base_rate_cents":   4500,
		"shuttles_per_hour": 3,
	}).Error; err != nil {
		t.Fatalf("configuring: %v", err)
	}

	if err := database.Migrate(); err != nil {
		t.Fatalf("migrating: %v", err)
	}

	var club models.Club
	if err := database.DB.First(&club).Error; err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if club.BaseRateCents != 4500 {
		t.Errorf("base rate = %d, want the configured 4500", club.BaseRateCents)
	}
	if club.ShuttlesPerHour != 3 {
		t.Errorf("shuttles per hour = %v, want the configured 3", club.ShuttlesPerHour)
	}
}
