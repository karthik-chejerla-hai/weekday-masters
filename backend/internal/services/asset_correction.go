package services

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AssetPurchaseView struct {
	models.Transaction
	AmountCents int64  `json:"amount_cents"`
	Units       int    `json:"units"`
	Reversed    bool   `json:"reversed"`
	ActorName   string `json:"actor_name"`
}

// AssetPurchases pages financial events, not individual ledger legs.
func (s *LedgerService) AssetPurchases(limit, offset int) ([]AssetPurchaseView, int64, error) {
	const from = ` FROM transactions t JOIN ledger_entries e ON e.transaction_id = t.id
 JOIN accounts a ON a.id = e.account_id
 LEFT JOIN users u ON u.id = t.created_by
 WHERE (t.kind = 'shuttle_purchase' AND a.kind = 'shuttle_stock')
 OR (t.kind = 'court_credit_purchase' AND a.kind = 'court_credit')`
	var total int64
	if err := database.DB.Raw("SELECT COUNT(*)" + from).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	items := []AssetPurchaseView{}
	err := database.DB.Raw(`SELECT t.*, COALESCE(NULLIF(u.nickname, ''), split_part(u.name, ' ', 1), 'Unknown recorder') AS actor_name, e.amount_cents, COALESCE(e.units, 0) AS units,
 EXISTS (SELECT 1 FROM transactions r WHERE r.reverses_transaction_id = t.id) AS reversed`+from+
		` ORDER BY t.occurred_at DESC, t.created_at DESC, t.id DESC LIMIT ? OFFSET ?`, limit, offset).Scan(&items).Error
	return items, total, err
}

type AssetPurchaseCorrectionInput struct {
	TransactionID uuid.UUID
	AmountCents   int64
	Units         int
	Reason        string
	CreatedBy     uuid.UUID
}

// CorrectAssetPurchase appends an exact reversal and a linked replacement. The
// unique original link also makes identical retries safe after a lost response.
func (s *LedgerService) CorrectAssetPurchase(in AssetPurchaseCorrectionInput) (*models.Transaction, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if in.AmountCents <= 0 || in.AmountCents > 4503599627370495 || in.Units < 0 || in.Units > 2147483647 {
		return nil, errors.New("enter a positive amount in integer cents and a valid shuttle count")
	}
	if in.Reason == "" || utf8.RuneCountInString(in.Reason) > 1000 {
		return nil, errors.New("a reason of 1–1000 characters is required")
	}
	var result *models.Transaction
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var actor models.User
		if err := tx.First(&actor, "id = ?", in.CreatedBy).Error; err != nil {
			return err
		}
		if !actor.IsAdmin() || !actor.IsApproved() {
			return newLedgerError("forbidden", 403, "Only approved admins can correct asset purchases.")
		}
		var original models.Transaction
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Entries").First(&original, "id = ?", in.TransactionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newLedgerError("not_found", 404, "Purchase not found.")
			}
			return err
		}
		assetKind := models.AccountKindCourtCredit
		switch original.Kind {
		case models.TxnShuttlePurchase:
			assetKind = models.AccountKindShuttleStock
			if in.Units <= 0 {
				return errors.New("shuttle count must be greater than zero")
			}
		case models.TxnCourtCreditPurchase:
			if in.Units != 0 {
				return errors.New("court credit has no shuttle count")
			}
		default:
			return errors.New("only shuttle and court credit purchases can be corrected here; use a balance audit for bank or court balances")
		}
		bank, err := s.ClubAccountID(tx, models.AccountKindBank)
		if err != nil {
			return err
		}
		asset, err := s.ClubAccountID(tx, assetKind)
		if err != nil {
			return err
		}
		movements := []Movement{{AccountID: bank, AmountCents: -in.AmountCents}, {AccountID: asset, AmountCents: in.AmountCents}}
		if original.Kind == models.TxnShuttlePurchase {
			movements[1].Units = &in.Units
		}
		if err := lockAccounts(tx, movements); err != nil {
			return err
		}
		var existing models.Transaction
		err = tx.Preload("Entries").First(&existing, "corrects_transaction_id = ?", original.ID).Error
		if err == nil {
			matches := existing.CreatedBy == in.CreatedBy && existing.CorrectionReason == in.Reason
			for _, e := range existing.Entries {
				if e.AccountID == asset {
					matches = matches && e.AmountCents == in.AmountCents && ((e.Units == nil && in.Units == 0) || (e.Units != nil && *e.Units == in.Units))
				}
			}
			if !matches {
				return newLedgerError("purchase_corrected", 409, "This purchase has already been corrected. Refresh purchase history.")
			}
			result = &existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var reversed int64
		if err := tx.Model(&models.Transaction{}).Where("reverses_transaction_id = ?", original.ID).Count(&reversed).Error; err != nil {
			return err
		}
		if reversed > 0 {
			return newLedgerError("purchase_reversed", 409, "This purchase has already been reversed. Refresh purchase history.")
		}
		var pending int64
		if err := tx.Model(&models.SplitwiseImport{}).Where("assets_confirmed = false").Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			return newLedgerError("assets_pending", 409, "Confirm imported assets before correcting purchases.")
		}
		if len(original.Entries) != 2 {
			return errors.New("purchase does not have the expected asset movements")
		}
		oldAmount := int64(0)
		oldUnits := 0
		undo := make([]Movement, 0, 2)
		for _, e := range original.Entries {
			if e.AccountID != bank && e.AccountID != asset {
				return errors.New("purchase has unexpected accounts")
			}
			if e.AccountID == asset {
				oldAmount = e.AmountCents
				if e.Units != nil {
					oldUnits = *e.Units
				}
			}
			m := Movement{AccountID: e.AccountID, AmountCents: -e.AmountCents}
			if e.Units != nil {
				units := -*e.Units
				m.Units = &units
			}
			undo = append(undo, m)
		}
		if oldAmount == in.AmountCents && oldUnits == in.Units {
			return errors.New("the corrected purchase already matches the recorded purchase")
		}
		if _, err := s.PostWithin(tx, PostInput{Kind: models.TxnReversal, ReversesTransactionID: &original.ID, CreatedBy: in.CreatedBy,
			Description: "Purchase correction: " + in.Reason, Movements: undo}); err != nil {
			return err
		}
		result, err = s.PostWithin(tx, PostInput{Kind: original.Kind, CorrectsTransactionID: &original.ID, CorrectionReason: in.Reason,
			Description: original.Description, OccurredAt: original.OccurredAt, CreatedBy: in.CreatedBy, Movements: movements})
		if err != nil {
			return err
		}
		if assetKind == models.AccountKindShuttleStock {
			stock, err := s.StockPosition(tx)
			if err != nil {
				return err
			}
			if stock.Units < 0 || stock.ValueCents < 0 || (stock.Units == 0 && stock.ValueCents != 0) {
				return newLedgerError("invalid_stock", 409, "This correction would leave invalid shuttle stock. Check the purchase against stock already consumed.")
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
