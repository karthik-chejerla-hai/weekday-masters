package services

import (
	"context"
	"errors"
	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"strings"
	"sync"
	"testing"
	"time"
)

const deviceSecret = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestDeviceLegacyPreservationMigrationAndAdoption(t *testing.T) {
	requireDB(t)
	owner := newUser(t, "owner")
	ns := &NotificationService{}
	row := models.UserPushToken{UserID: owner.ID, Token: "legacy"}
	if err := database.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ns.UpdateUserPreferences(owner.ID, map[string]interface{}{"push_enabled": false, "push_admin_announcements": false}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := database.Migrate(); err != nil {
			t.Fatal(err)
		}
	}
	var retained models.UserPushToken
	if err := database.DB.First(&retained, "id = ?", row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if retained.Token != row.Token || retained.UserID != row.UserID || retained.RevokedAt != nil || retained.Generation != 1 {
		t.Fatalf("changed old row %+v", retained)
	}
	installation := uuid.New()
	got, err := ns.AdoptPushDevice(owner.ID, installation, row.Token, deviceSecret)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != row.ID || got.Generation != 1 || got.State != "active" {
		t.Fatal(got)
	}
	if _, err := ns.AdoptPushDevice(owner.ID, uuid.New(), "absent", deviceSecret); !errors.Is(err, ErrDeviceNotFound) {
		t.Fatal(err)
	}
	other := newUser(t, "other")
	if _, err := ns.AdoptPushDevice(other.ID, uuid.New(), row.Token, deviceSecret); !errors.Is(err, ErrDeviceConflict) {
		t.Fatal(err)
	}
}
func TestDeviceRevocationFencesRefreshEnrollAndProofRetries(t *testing.T) {
	requireDB(t)
	owner := newUser(t, "owner")
	ns := &NotificationService{}
	installation := uuid.New()
	device, err := ns.EnrollPushDevice(owner.ID, installation, "initial", deviceSecret, "Phone", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := ns.RevokePushDevice(owner.ID, device.ID, device.Generation); err != nil {
		t.Fatal(err)
	}
	if err := ns.RevokePushDevice(owner.ID, device.ID, device.Generation); err != nil {
		t.Fatal(err)
	}
	if _, err := ns.RefreshPushDevice(owner.ID, device.ID, installation, "delayed", device.Generation); !errors.Is(err, ErrDeviceConflict) {
		t.Fatal(err)
	}
	if _, err := ns.EnrollPushDevice(owner.ID, installation, "delayed", deviceSecret, "", device.Generation); !errors.Is(err, ErrDeviceConflict) {
		t.Fatal(err)
	}
	renewed, err := ns.EnrollPushDevice(owner.ID, installation, "new", deviceSecret, "", device.Generation+1)
	if err != nil {
		t.Fatal(err)
	}
	if renewed.State != "active" || renewed.Generation != 3 {
		t.Fatal(renewed)
	}
	if err := ns.RevokePushDeviceProof(installation, deviceSecret, device.Generation); !errors.Is(err, ErrDeviceConflict) {
		t.Fatal("stale proof revoked new enrollment", err)
	}
	if err := ns.RevokePushDeviceProof(installation, strings.Repeat("00", 32), renewed.Generation); !errors.Is(err, ErrDeviceNotFound) {
		t.Fatal(err)
	}
	if err := ns.RevokePushDeviceProof(installation, deviceSecret, renewed.Generation); err != nil {
		t.Fatal(err)
	}
	other := newUser(t, "other")
	if _, err := ns.EnrollPushDevice(other.ID, installation, "other", deviceSecret, "", 0); !errors.Is(err, ErrDeviceConflict) {
		t.Fatal(err)
	}
	if _, err := ns.EnrollPushDevice(other.ID, uuid.New(), "other", strings.Repeat("ab", 32), "", 0); err != nil {
		t.Fatal(err)
	}
}
func TestConcurrentDisableCannotBeUndoneByRefresh(t *testing.T) {
	requireDB(t)
	owner := newUser(t, "owner")
	ns := &NotificationService{}
	installation := uuid.New()
	device, err := ns.EnrollPushDevice(owner.ID, installation, "initial", deviceSecret, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, _ = ns.RefreshPushDevice(owner.ID, device.ID, installation, "rotated", device.Generation)
	}()
	go func() {
		defer wg.Done()
		<-start
		err := ns.RevokePushDeviceProof(installation, deviceSecret, device.Generation)
		if errors.Is(err, ErrDeviceConflict) {
			err = ns.RevokePushDeviceProof(installation, deviceSecret, device.Generation+1)
		}
		if err != nil {
			t.Error(err)
		}
	}()
	close(start)
	wg.Wait()
	var row models.UserPushToken
	database.DB.First(&row, "id = ?", device.ID)
	if row.RevokedAt == nil {
		t.Fatal("refresh undid revocation")
	}
	if _, err := ns.RefreshPushDevice(owner.ID, device.ID, installation, "late", row.Generation); !errors.Is(err, ErrDeviceConflict) {
		t.Fatal(err)
	}
}
func TestDeviceDeliveryIgnoresFlagsAndExcludesRevokedInvalidated(t *testing.T) {
	requireDB(t)
	owner := newUser(t, "owner")
	now := time.Now()
	for _, row := range []models.UserPushToken{{UserID: owner.ID, Token: "active"}, {UserID: owner.ID, Token: "revoked", RevokedAt: &now}, {UserID: owner.ID, Token: "invalid", InvalidatedAt: &now}} {
		if err := database.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	ns := &NotificationService{fcmEnabled: true, fcmClient: stubPushClient(func(_ context.Context, msg *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
		if len(msg.Tokens) != 1 || msg.Tokens[0] != "active" {
			t.Fatalf("tokens %v", msg.Tokens)
		}
		return &messaging.BatchResponse{SuccessCount: 1, Responses: []*messaging.SendResponse{{Success: true}}}, nil
	})}
	if _, err := ns.UpdateUserPreferences(owner.ID, map[string]interface{}{"push_enabled": false, "push_admin_announcements": false}); err != nil {
		t.Fatal(err)
	}
	if err := ns.SendNotification(context.Background(), owner.ID, models.NotificationAdminAnnouncement, "Title", "Body", nil); err != nil {
		t.Fatal(err)
	}
}
func TestInvalidatedEnrollmentCanRefreshWithoutRevivingRevocation(t *testing.T) {
	requireDB(t)
	owner := newUser(t, "owner")
	ns := &NotificationService{}
	installation := uuid.New()
	device, err := ns.EnrollPushDevice(owner.ID, installation, "initial", deviceSecret, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Model(&models.UserPushToken{}).Where("id = ?", device.ID).Update("invalidated_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	repaired, err := ns.RefreshPushDevice(owner.ID, device.ID, installation, "repaired", 1)
	if err != nil || repaired.State != "active" || repaired.Generation != 2 {
		t.Fatal(repaired, err)
	}
	if _, err := ns.RefreshPushDevice(owner.ID, device.ID, installation, "repaired", 1); err != nil {
		t.Fatal("identical retry", err)
	}
	if _, err := ns.RefreshPushDevice(owner.ID, device.ID, installation, "stale", 1); !errors.Is(err, ErrDeviceConflict) {
		t.Fatal(err)
	}
}

func TestRevokeProofRecoveryDisclosesCounterOnlyAfterValidProof(t *testing.T) {
	requireDB(t)
	owner := newUser(t, "owner")
	ns := &NotificationService{}
	installation := uuid.New()
	device, err := ns.EnrollPushDevice(owner.ID, installation, "address", deviceSecret, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ns.RefreshPushDevice(owner.ID, device.ID, installation, "rotated", 1)
	if err != nil {
		t.Fatal(err)
	}
	err = ns.RevokePushDeviceProof(installation, strings.Repeat("00", 32), 0)
	var counter *DeviceGenerationConflictError
	if !errors.Is(err, ErrDeviceNotFound) || errors.As(err, &counter) {
		t.Fatal("invalid proof disclosed counter", err)
	}
	err = ns.RevokePushDeviceProof(installation, deviceSecret, 0)
	if !errors.As(err, &counter) || counter.CurrentGeneration != 2 {
		t.Fatal("valid lost-response proof could not recover", err)
	}
	if err := ns.RevokePushDeviceProof(installation, deviceSecret, counter.CurrentGeneration); err != nil {
		t.Fatal(err)
	}
}
func TestForeignLegacyCleanupPreservesSameOwnerAndAdoptedRows(t *testing.T) {
	requireDB(t)
	owner := newUser(t, "owner")
	other := newUser(t, "other")
	ns := &NotificationService{}
	row := models.UserPushToken{UserID: owner.ID, Token: "legacy-address"}
	if err := database.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	kept, err := ns.RevokeForeignLegacyToken(owner.ID, row.Token)
	if err != nil || kept.CleanupRequired {
		t.Fatal(kept, err)
	}
	retired, err := ns.RevokeForeignLegacyToken(other.ID, row.Token)
	if err != nil || !retired.CleanupRequired {
		t.Fatal(retired, err)
	}
	var stored models.UserPushToken
	database.DB.First(&stored, "id = ?", row.ID)
	if stored.UserID != owner.ID || stored.Token != row.Token || stored.RevokedAt == nil || stored.Generation != 2 || stored.InstallationID != nil {
		t.Fatal("legacy token transferred or lost", stored)
	}
	if _, err := ns.RevokeForeignLegacyToken(other.ID, row.Token); err != nil {
		t.Fatal(err)
	}
	database.DB.First(&stored, "id = ?", row.ID)
	if stored.Generation != 2 {
		t.Fatal("duplicate legacy cleanup advanced generation")
	}
	adopted, err := ns.EnrollPushDevice(owner.ID, uuid.New(), "adopted-address", deviceSecret, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	untouched, err := ns.RevokeForeignLegacyToken(other.ID, "adopted-address")
	if err != nil || untouched.CleanupRequired {
		t.Fatal(untouched, err)
	}
	var healthy models.UserPushToken
	if err := database.DB.First(&healthy, "id = ?", adopted.ID).Error; err != nil {
		t.Fatal(err)
	}
	if healthy.RevokedAt != nil {
		t.Fatal("legacy endpoint revoked v2 installation")
	}
}
