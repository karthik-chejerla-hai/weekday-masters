package services

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
)

type BalanceCorrectionInput struct {
	RequestID     uuid.UUID
	AccountKind   models.AccountKind
	ExpectedCents int64
	TargetCents   int64
	Reason        string
	CreatedBy     uuid.UUID
}

// CorrectAssetBalance records a reconciliation against surplus, preserving cash
// in the other accounts and every original entry. All checks and audit writes
// share the ledger transaction and its canonical account locking order.
func (s *LedgerService) CorrectAssetBalance(in BalanceCorrectionInput) (*models.BalanceCorrection, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if in.RequestID == uuid.Nil || (in.AccountKind != models.AccountKindCourtCredit && in.AccountKind != models.AccountKindBank) {
		return nil, errors.New("select bank or court credit and provide a request ID")
	}
	// Bound to exact integers supported by the JSON client; keeps arithmetic safe.
	const maxCents int64 = 4503599627370495
	if in.TargetCents < 0 || in.TargetCents > maxCents || in.ExpectedCents < -maxCents || in.ExpectedCents > maxCents {
		return nil, errors.New("target must be a non-negative balance in integer cents")
	}
	if in.Reason == "" || utf8.RuneCountInString(in.Reason) > 1000 {
		return nil, errors.New("a reason of 1–1000 characters is required")
	}
	var result models.BalanceCorrection
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var actor models.User
		if err := tx.First(&actor, "id = ?", in.CreatedBy).Error; err != nil {
			return err
		}
		if !actor.IsAdmin() || !actor.IsApproved() {
			return newLedgerError("forbidden", 403, "Only approved admins can correct balances.")
		}
		asset, err := s.ClubAccountID(tx, in.AccountKind)
		if err != nil {
			return err
		}
		surplus, err := s.ClubAccountID(tx, models.AccountKindSurplus)
		if err != nil {
			return err
		}
		movements := []Movement{{AccountID: asset}, {AccountID: surplus}}
		if err := lockAccounts(tx, movements); err != nil {
			return err
		}
		// Every correction locks surplus before this lookup, serializing retries.
		err = tx.First(&result, "request_id = ?", in.RequestID).Error
		if err == nil {
			if result.AccountKind != in.AccountKind || result.BeforeCents != in.ExpectedCents || result.AfterCents != in.TargetCents || result.Reason != in.Reason || result.CreatedBy != in.CreatedBy {
				return newLedgerError("correction_request_conflict", 409, "This request ID was already used for another correction.")
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var pending int64
		if err := tx.Model(&models.SplitwiseImport{}).Where("assets_confirmed = false").Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			return newLedgerError("assets_pending", 409, "Confirm imported assets before correcting balances.")
		}
		before, err := s.BalanceOfKind(tx, in.AccountKind)
		if err != nil {
			return err
		}
		if before != in.ExpectedCents {
			return newLedgerError("balance_changed", 409, "The balance changed. Refresh club assets and review the correction again.")
		}
		if before == in.TargetCents {
			return errors.New("the target balance already matches the recorded balance")
		}
		delta := in.TargetCents - before
		movements[0].AmountCents, movements[1].AmountCents = delta, delta
		posted, err := s.PostWithin(tx, PostInput{
			Kind: models.TxnBalanceCorrection, CreatedBy: in.CreatedBy,
			Description: fmt.Sprintf("%s balance correction: %d → %d cents (adjustment %+d cents). Reason: %s", in.AccountKind, before, in.TargetCents, delta, in.Reason),
			Movements:   movements,
		})
		if err != nil {
			return err
		}
		result = models.BalanceCorrection{RequestID: in.RequestID, TransactionID: posted.ID, AccountID: asset,
			AccountKind: in.AccountKind, BeforeCents: before, AfterCents: in.TargetCents, AdjustmentCents: delta,
			Reason: in.Reason, CreatedBy: in.CreatedBy, ActorName: actor.DisplayName(), CreatedAt: posted.CreatedAt}
		return tx.Create(&result).Error
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *LedgerService) ListBalanceCorrections(limit, offset int) ([]models.BalanceCorrection, int64, error) {
	var items []models.BalanceCorrection
	var total int64
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.BalanceCorrection{}).Count(&total).Error; err != nil {
			return err
		}
		return tx.Order("created_at DESC, request_id DESC").Limit(limit).Offset(offset).Find(&items).Error
	})
	return items, total, err
}
