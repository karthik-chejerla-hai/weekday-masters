package services

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

var ErrDeviceConflict = errors.New("device registration changed; refresh setup before trying again")
var ErrDeviceNotFound = errors.New("device registration unavailable")

// Returned only after the revoke-only device proof has been verified. It
// discloses a counter, never owner identity, addresses or enrollment metadata.
type DeviceGenerationConflictError struct{ CurrentGeneration int64 }

func (e *DeviceGenerationConflictError) Error() string { return ErrDeviceConflict.Error() }
func (e *DeviceGenerationConflictError) Unwrap() error { return ErrDeviceConflict }

type LegacyPushCleanupResult struct {
	CleanupRequired bool `json:"cleanup_required"`
}

// Legacy token possession permits revoke-only cleanup during the first account
// switch after rollout. Same-owner rows are preserved; adopted installations
// cannot be modified through this compatibility cleanup endpoint. An adopted
// address still requires browser deletion when its local installation proof is
// lost, including when the caller is its owner.
func (s *NotificationService) RevokeForeignLegacyToken(actorID uuid.UUID, token string) (*LegacyPushCleanupResult, error) {
	result := &LegacyPushCleanupResult{}
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var row models.UserPushToken
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token = ?", token).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if row.InstallationID != nil {
			result.CleanupRequired = true
			return nil
		}
		if row.UserID == actorID && row.RevokedAt == nil {
			return nil
		}
		result.CleanupRequired = true
		if row.RevokedAt != nil {
			return nil
		}
		return tx.Model(&row).Updates(map[string]interface{}{"revoked_at": time.Now(), "generation": gorm.Expr("generation + 1")}).Error
	})
	return result, err
}

type PushEnrollment struct {
	ID             uuid.UUID  `json:"id"`
	InstallationID *uuid.UUID `json:"installation_id"`
	Generation     int64      `json:"generation"`
	DeviceName     string     `json:"device_name"`
	State          string     `json:"state"`
}

func enrollment(t models.UserPushToken) PushEnrollment {
	state := "active"
	if t.RevokedAt != nil {
		state = "revoked"
	} else if t.InvalidatedAt != nil {
		state = "invalidated"
	}
	return PushEnrollment{t.ID, t.InstallationID, t.Generation, t.DeviceName, state}
}
func secretHash(secret string) (string, error) {
	bytes, err := hex.DecodeString(secret)
	if err != nil || len(bytes) != 32 {
		return "", ErrDeviceConflict
	}
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:]), nil
}
func proofMatches(stored, provided string) bool {
	return len(stored) == 64 && subtle.ConstantTimeCompare([]byte(stored), []byte(provided)) == 1
}
func deviceLock(tx *gorm.DB, installation uuid.UUID) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "push-installation:"+installation.String()).Error
}
func (s *NotificationService) GetPushDevices(userID uuid.UUID) ([]PushEnrollment, error) {
	var rows []models.UserPushToken
	if err := database.DB.Where("user_id = ?", userID).Order("created_at,id").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]PushEnrollment, 0, len(rows))
	for _, row := range rows {
		result = append(result, enrollment(row))
	}
	return result, nil
}

// AdoptPushDevice only attaches metadata to the caller's existing legacy token.
// It neither enrolls absent tokens nor revives a revoked device.
func (s *NotificationService) AdoptPushDevice(userID, installation uuid.UUID, token, secret string) (*PushEnrollment, error) {
	return s.enrollPushDevice(userID, installation, token, secret, "", 0, true)
}
func (s *NotificationService) EnrollPushDevice(userID, installation uuid.UUID, token, secret, name string, generation int64) (*PushEnrollment, error) {
	return s.enrollPushDevice(userID, installation, token, secret, name, generation, false)
}
func (s *NotificationService) enrollPushDevice(userID, installation uuid.UUID, token, secret, name string, generation int64, adoptOnly bool) (*PushEnrollment, error) {
	hash, err := secretHash(secret)
	if err != nil {
		return nil, err
	}
	var row models.UserPushToken
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := deviceLock(tx, installation); err != nil {
			return err
		}
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("installation_id = ?", installation).First(&row).Error
		if err == nil {
			if row.UserID != userID || !proofMatches(row.RevocationHash, hash) {
				return ErrDeviceConflict
			}
			if row.RevokedAt == nil && row.InvalidatedAt == nil && row.Token == token {
				return nil
			} // identical enrollment retry, no state change
			if adoptOnly || generation != row.Generation {
				return ErrDeviceConflict
			}
			// Only explicit enrollment may revive revocation. The current generation
			// and device proof fence a delayed request sent before Disable.
			row.Token = token
			row.DeviceName = name
			row.Generation++
			row.RevokedAt = nil
			row.InvalidatedAt = nil
			row.LastUsedAt = time.Now()
			return tx.Save(&row).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if generation != 0 {
			return ErrDeviceConflict
		}
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token = ?", token).First(&row).Error
		if err == nil {
			if row.UserID != userID || row.InstallationID != nil || row.RevokedAt != nil {
				return ErrDeviceConflict
			}
			row.InstallationID = &installation
			row.RevocationHash = hash
			row.LastUsedAt = time.Now()
			return tx.Save(&row).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if adoptOnly {
			return ErrDeviceNotFound
		}
		row = models.UserPushToken{UserID: userID, InstallationID: &installation, Token: token, DeviceName: name, Generation: 1, RevocationHash: hash}
		return tx.Create(&row).Error
	})
	if err != nil {
		return nil, err
	}
	result := enrollment(row)
	return &result, nil
}
func (s *NotificationService) RefreshPushDevice(userID, id, installation uuid.UUID, token string, generation int64) (*PushEnrollment, error) {
	var row models.UserPushToken
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := deviceLock(tx, installation); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ? AND installation_id = ?", id, userID, installation).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrDeviceNotFound
			}
			return err
		}
		if row.RevokedAt != nil {
			return ErrDeviceConflict
		}
		if row.Token == token && row.InvalidatedAt == nil {
			return nil
		}
		if row.Generation != generation {
			return ErrDeviceConflict
		}
		row.Token = token
		row.Generation++
		row.InvalidatedAt = nil
		row.LastUsedAt = time.Now()
		return tx.Save(&row).Error
	})
	if err != nil {
		return nil, err
	}
	result := enrollment(row)
	return &result, nil
}

// RevokePushDeviceProof is revoke-only. Device proof permits account-switch
// cleanup after the former JWT is unavailable; it never reads or transfers ownership.
func (s *NotificationService) RevokePushDeviceProof(installation uuid.UUID, secret string, generation int64) error {
	hash, err := secretHash(secret)
	if err != nil {
		return ErrDeviceNotFound
	}
	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := deviceLock(tx, installation); err != nil {
			return err
		}
		var row models.UserPushToken
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("installation_id = ?", installation).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrDeviceNotFound
			}
			return err
		}
		if !proofMatches(row.RevocationHash, hash) {
			return ErrDeviceNotFound
		}
		if row.RevokedAt != nil {
			return nil
		}
		if row.Generation != generation {
			return &DeviceGenerationConflictError{CurrentGeneration: row.Generation}
		}
		now := time.Now()
		return tx.Model(&row).Updates(map[string]interface{}{"revoked_at": now, "generation": row.Generation + 1}).Error
	})
}
func (s *NotificationService) RevokePushDevice(userID, id uuid.UUID, generation int64) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var row models.UserPushToken
		// Find installation before taking the shared ordering lock, then re-read.
		if err := tx.Where("id = ? AND user_id = ?", id, userID).First(&row).Error; err != nil {
			return ErrDeviceNotFound
		}
		if row.InstallationID == nil {
			return ErrDeviceConflict
		}
		if err := deviceLock(tx, *row.InstallationID); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", id, userID).First(&row).Error; err != nil {
			return ErrDeviceNotFound
		}
		if row.RevokedAt != nil {
			return nil
		}
		if row.Generation != generation {
			return ErrDeviceConflict
		}
		now := time.Now()
		return tx.Model(&row).Updates(map[string]interface{}{"revoked_at": now, "generation": row.Generation + 1}).Error
	})
}
