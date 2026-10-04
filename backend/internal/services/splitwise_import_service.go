package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/splitwise"
	"gorm.io/gorm"
)

type ImportAssets struct {
	BankCents        int64 `json:"bank_cents"`
	CourtCreditCents int64 `json:"court_credit_cents"`
	ShuttleUnits     int   `json:"shuttle_units"`
	ShuttleCents     int64 `json:"shuttle_cents"`
}

type ImportBalance struct {
	Name          string `json:"name"`
	Inactive      bool   `json:"inactive"`
	ExpectedCents int64  `json:"expected_cents"`
	ActualCents   int64  `json:"actual_cents"`
}

type SplitwiseImportReport struct {
	ImportID            uuid.UUID       `json:"import_id"`
	Reused              bool            `json:"reused"`
	Transactions        int             `json:"transactions"`
	Sessions            int             `json:"sessions"`
	VerifiedCells       int             `json:"verified_cells"`
	UsersCreated        int             `json:"users_created"`
	NotificationsPaused bool            `json:"notifications_paused"`
	AssetsPending       bool            `json:"assets_pending"`
	ResidualCents       int64           `json:"residual_cents"`
	Balances            []ImportBalance `json:"balances"`
	Warnings            []string        `json:"warnings"`
}

// ImportSplitwise creates all source records, users and ledger movements in one
// transaction. It never uses the seed package or sends a notification.
func ImportSplitwise(source *splitwise.Export, mapping splitwise.Mapping, assets *ImportAssets) (*SplitwiseImportReport, error) {
	return ImportSplitwiseWithOptions(source, mapping, assets, SplitwiseImportOptions{})
}

type SplitwiseImportOptions struct {
	// Allows only top-ups whose exact reversals are retained in the ledger.
	AllowReversedTopups bool
}

func ImportSplitwiseWithOptions(source *splitwise.Export, mapping splitwise.Mapping, assets *ImportAssets, options SplitwiseImportOptions) (*SplitwiseImportReport, error) {
	if err := source.ValidateMapping(mapping); err != nil {
		return nil, err
	}
	if assets != nil {
		if err := validateImportAssets(*assets); err != nil {
			return nil, err
		}
	}
	report := &SplitwiseImportReport{}
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Serializes two import commands, including the empty-ledger check.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(724359681)").Error; err != nil {
			return err
		}
		// An app posting a transaction during first import must not pass the
		// empty-ledger check unnoticed. This also protects the asset snapshot.
		if err := tx.Exec("LOCK TABLE transactions IN SHARE ROW EXCLUSIVE MODE").Error; err != nil {
			return err
		}
		var batch models.SplitwiseImport
		err := tx.First(&batch).Error
		if err == nil {
			if batch.SourceHash != source.Hash || batch.MappingHash != mapping.Hash() {
				return fmt.Errorf("a different import already exists; refusing overlapping history")
			}
			report.Reused = true
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		} else {
			var count int64
			if err := tx.Model(&models.Transaction{}).Count(&count).Error; err != nil {
				return err
			}
			if count != 0 {
				if !options.AllowReversedTopups {
					return fmt.Errorf("first import requires an empty ledger; existing entries must be reviewed")
				}
				if err := verifyReversedTopups(tx); err != nil {
					return err
				}
			}
			batch = models.SplitwiseImport{SourceHash: source.Hash, MappingHash: mapping.Hash(), Cutoff: source.Cutoff}
			if err := tx.Create(&batch).Error; err != nil {
				return err
			}
			participants, actor, created, err := importParticipants(tx, source, mapping, batch.ID)
			if err != nil {
				return err
			}
			report.UsersCreated = created
			if err := importSourceRows(tx, source, mapping, batch.ID, actor, participants); err != nil {
				return err
			}
		}
		// A retained pause is required even when the caller re-runs a verified
		// import. No credentials or per-user settings need to be removed.
		var club models.Club
		if err := tx.First(&club).Error; err != nil {
			return err
		}
		if err := tx.Model(&club).Update("notifications_paused", true).Error; err != nil {
			return err
		}
		if assets != nil {
			if err := recordImportAssets(tx, &batch, mapping, *assets, options); err != nil {
				return err
			}
		}
		if err := verifySplitwise(tx, source, mapping, batch, report); err != nil {
			return err
		}
		report.NotificationsPaused = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	return report, nil
}

func importParticipants(tx *gorm.DB, source *splitwise.Export, mapping splitwise.Mapping, batchID uuid.UUID) (map[int]models.SplitwiseParticipant, uuid.UUID, int, error) {
	members := map[string]splitwise.Member{}
	for _, m := range mapping.Members {
		members[m.SourceName] = m
	}
	participants := map[int]models.SplitwiseParticipant{}
	var actor uuid.UUID
	created := 0
	for i, name := range source.Names {
		if i == source.ClubIndex {
			continue
		}
		p := models.SplitwiseParticipant{ImportID: batchID, SourceName: name, ClosingCents: source.Totals[i]}
		account := models.Account{Kind: models.AccountKindPlayer, Name: strings.TrimSuffix(name, " (removed)") + " (inactive)"}
		if m, ok := members[name]; ok {
			var matches []models.User
			if err := tx.Where("LOWER(email) = ?", m.Email).Find(&matches).Error; err != nil {
				return nil, actor, created, err
			}
			if len(matches) > 1 {
				return nil, actor, created, fmt.Errorf("email matches multiple users")
			}
			var user models.User
			if len(matches) == 1 {
				user = matches[0]
				if !user.IsApproved() {
					return nil, actor, created, fmt.Errorf("mapped user %s is not approved", name)
				}
			} else {
				role := models.RolePlayer
				if m.Email == mapping.AdminEmail {
					role = models.RoleAdmin
				}
				user = models.User{Name: m.Name, Email: m.Email, Auth0ID: models.NewInvitePlaceholder(), MembershipStatus: models.MembershipApproved, Role: role, IsPlayer: true}
				if err := tx.Create(&user).Error; err != nil {
					return nil, actor, created, err
				}
				created++
			}
			if m.Email == mapping.AdminEmail {
				if !user.IsAdmin() {
					return nil, actor, created, fmt.Errorf("import actor must already be an admin")
				}
				actor = user.ID
			}
			p.UserID = &user.ID
			account.UserID = &user.ID
			account.Name = user.DisplayName()
			var existing models.Account
			err := tx.Where("user_id = ?", user.ID).First(&existing).Error
			if err == nil {
				account = existing
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, actor, created, err
			}
		}
		if account.ID == uuid.Nil {
			if err := tx.Create(&account).Error; err != nil {
				return nil, actor, created, err
			}
		}
		p.AccountID = account.ID
		if err := tx.Create(&p).Error; err != nil {
			return nil, actor, created, err
		}
		participants[i] = p
	}
	return participants, actor, created, nil
}

func importSourceRows(tx *gorm.DB, source *splitwise.Export, mapping splitwise.Mapping, batchID, actor uuid.UUID, participants map[int]models.SplitwiseParticipant) error {
	ledger := NewLedgerService()
	surplus, err := ledger.ClubAccountID(tx, models.AccountKindSurplus)
	if err != nil {
		return err
	}
	for _, row := range source.Rows {
		playDate, basis := row.Date, "recorded"
		if row.IsSession {
			playDate, basis, err = splitwise.PlayDate(row)
			if err != nil {
				return err
			}
		}
		record := models.SplitwiseRecord{ID: uuid.New(), ImportID: batchID, RowNumber: row.Number, RecordedDate: row.Date, PlayedDate: playDate, DateBasis: basis, Description: row.Description, Category: row.Category, CostCents: row.CostCents, ClubCents: row.Amounts[source.ClubIndex], IsSession: row.IsSession}
		var movements []Movement
		for i, p := range participants {
			if row.Amounts[i] != 0 {
				movements = append(movements, Movement{AccountID: p.AccountID, AmountCents: row.Amounts[i]})
			}
		}
		// The source club pot is not evidence of bank funds or historic stock.
		// Surplus offsets the participant movements until assets are verified.
		if record.ClubCents != 0 {
			movements = append(movements, Movement{AccountID: surplus, AmountCents: record.ClubCents})
		}
		if len(movements) == 0 {
			movements = append(movements, Movement{AccountID: surplus})
		}
		var sessionID *uuid.UUID
		if row.IsSession {
			sessionID = &record.ID
		}
		// Stable order for multiple entries recorded on one source date. This
		// is import ordering metadata, not a claimed real transaction time.
		occurred := row.Date.Add(time.Duration(row.Number) * time.Microsecond)
		txn, err := ledger.PostWithin(tx, PostInput{Kind: models.TxnSplitwiseImport, SessionID: sessionID, Description: row.Description, OccurredAt: occurred, CreatedBy: actor, Movements: movements})
		if err != nil {
			return err
		}
		record.TransactionID = txn.ID
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		var charges, paid []int64
		if row.IsSession {
			charges, paid, err = source.Charges(row, mapping)
			if err != nil {
				return err
			}
		}
		for i, p := range participants {
			change := models.SplitwiseChange{RecordID: record.ID, ParticipantID: p.ID, NetCents: row.Amounts[i]}
			if row.IsSession {
				v := charges[i]
				change.ChargeCents = &v
				change.PaidCents = paid[i]
			}
			if err := tx.Create(&change).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func validateImportAssets(a ImportAssets) error {
	if a.BankCents < 0 || a.CourtCreditCents < 0 || a.ShuttleCents < 0 || a.ShuttleUnits < 0 || (a.ShuttleUnits == 0 && a.ShuttleCents != 0) || (a.ShuttleUnits > 0 && a.ShuttleCents == 0) {
		return fmt.Errorf("invalid asset snapshot; stock needs both positive units and value")
	}
	if a.BankCents > 1e12 || a.CourtCreditCents > 1e12 || a.ShuttleCents > 1e12 || a.ShuttleUnits > 1e8 {
		return fmt.Errorf("asset snapshot exceeds supported range")
	}
	return nil
}

func recordImportAssets(tx *gorm.DB, batch *models.SplitwiseImport, mapping splitwise.Mapping, a ImportAssets, options SplitwiseImportOptions) error {
	var other int64
	if err := tx.Model(&models.Transaction{}).Where("kind NOT IN ?", []models.TransactionKind{models.TxnSplitwiseImport, models.TxnImportAssets}).Count(&other).Error; err != nil {
		return err
	}
	if other > 0 {
		if !options.AllowReversedTopups {
			return fmt.Errorf("asset snapshot must be confirmed before new club transactions")
		}
		if err := verifyReversedTopups(tx); err != nil {
			return err
		}
	}
	ledger := NewLedgerService()
	amounts := []struct {
		kind  models.AccountKind
		cents int64
		units *int
	}{
		{models.AccountKindBank, a.BankCents, nil}, {models.AccountKindCourtCredit, a.CourtCreditCents, nil},
		{models.AccountKindShuttleStock, a.ShuttleCents, &a.ShuttleUnits}, {models.AccountKindSurplus, a.BankCents + a.CourtCreditCents + a.ShuttleCents, nil},
	}
	var movements []Movement
	for _, item := range amounts {
		id, err := ledger.ClubAccountID(tx, item.kind)
		if err != nil {
			return err
		}
		movements = append(movements, Movement{AccountID: id, AmountCents: item.cents, Units: item.units})
	}
	if batch.AssetsConfirmed {
		if batch.AssetTransactionID == nil {
			return fmt.Errorf("confirmed assets have no source transaction")
		}
		return verifyMovements(tx, *batch.AssetTransactionID, movements)
	}
	var actor models.User
	if err := tx.Where("email = ?", mapping.AdminEmail).First(&actor).Error; err != nil {
		return err
	}
	txn, err := ledger.PostWithin(tx, PostInput{Kind: models.TxnImportAssets, Description: "Verified club assets at Splitwise cutover", OccurredAt: batch.Cutoff.Add(23 * time.Hour), CreatedBy: actor.ID, Movements: movements})
	if err != nil {
		return err
	}
	batch.AssetsConfirmed = true
	batch.AssetTransactionID = &txn.ID
	return tx.Save(batch).Error
}

// verifyReversedTopups permits reviewed setup history without deleting it.
// Every existing top-up must have an exact reversal. Other financial events,
// orphan reversals and partially cancelled top-ups are refused.
func verifyReversedTopups(tx *gorm.DB) error {
	var transactions []models.Transaction
	if err := tx.Preload("Entries").Where("kind NOT IN ?", []models.TransactionKind{models.TxnSplitwiseImport, models.TxnImportAssets}).Find(&transactions).Error; err != nil {
		return err
	}
	originals := map[uuid.UUID]models.Transaction{}
	reversals := map[uuid.UUID]models.Transaction{}
	for _, transaction := range transactions {
		switch transaction.Kind {
		case models.TxnPlayerTopup:
			if transaction.ReversesTransactionID != nil || len(transaction.Entries) == 0 {
				return fmt.Errorf("invalid existing top-up")
			}
			originals[transaction.ID] = transaction
		case models.TxnReversal:
			if transaction.ReversesTransactionID == nil {
				return fmt.Errorf("existing reversal has no original transaction")
			}
			id := *transaction.ReversesTransactionID
			if _, exists := reversals[id]; exists {
				return fmt.Errorf("duplicate existing reversal")
			}
			reversals[id] = transaction
		default:
			return fmt.Errorf("only exactly reversed top-ups may precede the import")
		}
	}
	if len(originals) != len(reversals) {
		return fmt.Errorf("every existing top-up must have an exact reversal")
	}
	for id, original := range originals {
		reversal, exists := reversals[id]
		if !exists {
			return fmt.Errorf("existing top-up %s has not been reversed", id)
		}
		var expected []Movement
		for _, entry := range original.Entries {
			movement := Movement{AccountID: entry.AccountID, AmountCents: -entry.AmountCents}
			if entry.Units != nil {
				units := -*entry.Units
				movement.Units = &units
			}
			expected = append(expected, movement)
		}
		if err := verifyMovements(tx, reversal.ID, expected); err != nil {
			return err
		}
	}
	return nil
}

func verifyMovements(tx *gorm.DB, id uuid.UUID, want []Movement) error {
	var entries []models.LedgerEntry
	if err := tx.Where("transaction_id = ?", id).Find(&entries).Error; err != nil {
		return err
	}
	if len(entries) != len(want) {
		return fmt.Errorf("transaction %s has an unexpected entry count", id)
	}
	byAccount := map[uuid.UUID]models.LedgerEntry{}
	for _, e := range entries {
		if _, ok := byAccount[e.AccountID]; ok {
			return fmt.Errorf("duplicate account entry")
		}
		byAccount[e.AccountID] = e
	}
	for _, m := range want {
		e, ok := byAccount[m.AccountID]
		if !ok || e.AmountCents != m.AmountCents || (e.Units == nil) != (m.Units == nil) || (e.Units != nil && *e.Units != *m.Units) {
			return fmt.Errorf("transaction %s does not match source movements", id)
		}
	}
	return nil
}

func verifySplitwise(tx *gorm.DB, source *splitwise.Export, mapping splitwise.Mapping, batch models.SplitwiseImport, report *SplitwiseImportReport) error {
	report.ImportID = batch.ID
	report.AssetsPending = !batch.AssetsConfirmed
	var participants []models.SplitwiseParticipant
	if err := tx.Preload("Account").Preload("User").Where("import_id = ?", batch.ID).Find(&participants).Error; err != nil {
		return err
	}
	if len(participants) != len(source.Names)-1 {
		return fmt.Errorf("participant count differs from source")
	}
	byName := map[string]models.SplitwiseParticipant{}
	members := map[string]splitwise.Member{}
	for _, m := range mapping.Members {
		members[m.SourceName] = m
	}
	for _, p := range participants {
		byName[p.SourceName] = p
	}
	surplus, err := NewLedgerService().ClubAccountID(tx, models.AccountKindSurplus)
	if err != nil {
		return err
	}
	var records []models.SplitwiseRecord
	if err := tx.Preload("Changes").Preload("Transaction").Where("import_id = ?", batch.ID).Order("row_number").Find(&records).Error; err != nil {
		return err
	}
	if len(records) != len(source.Rows) {
		return fmt.Errorf("record count differs from source")
	}
	sessionDates := map[string]bool{}
	for i, r := range records {
		src := source.Rows[i]
		date, basis := src.Date, "recorded"
		if src.IsSession {
			date, basis, err = splitwise.PlayDate(src)
			if err != nil {
				return err
			}
		}
		if r.RowNumber != src.Number || r.Description != src.Description || r.Category != src.Category || r.CostCents != src.CostCents || r.ClubCents != src.Amounts[source.ClubIndex] || r.RecordedDate.Format("2006-01-02") != src.Date.Format("2006-01-02") || r.PlayedDate.Format("2006-01-02") != date.Format("2006-01-02") || r.DateBasis != basis || r.IsSession != src.IsSession {
			return fmt.Errorf("source metadata differs on row %d", src.Number)
		}
		if r.Transaction == nil || r.Transaction.Kind != models.TxnSplitwiseImport || r.Transaction.Description != src.Description || !r.Transaction.OccurredAt.Equal(src.Date.Add(time.Duration(src.Number)*time.Microsecond)) {
			return fmt.Errorf("ledger metadata differs on row %d", src.Number)
		}
		if src.IsSession && (r.Transaction.SessionID == nil || *r.Transaction.SessionID != r.ID) {
			return fmt.Errorf("session link differs")
		}
		if len(r.Changes) != len(participants) {
			return fmt.Errorf("source cell count differs on row %d", src.Number)
		}
		changes := map[uuid.UUID]models.SplitwiseChange{}
		for _, c := range r.Changes {
			changes[c.ParticipantID] = c
		}
		var charges, paid []int64
		if src.IsSession {
			charges, paid, err = source.Charges(src, mapping)
			if err != nil {
				return err
			}
			key := date.Format("2006-01-02")
			if !sessionDates[key] {
				report.Sessions++
				sessionDates[key] = true
			}
		}
		var expected []Movement
		for j, name := range source.Names {
			if j == source.ClubIndex {
				continue
			}
			p, ok := byName[name]
			if !ok {
				return fmt.Errorf("missing participant %q", name)
			}
			c, ok := changes[p.ID]
			if !ok || c.NetCents != src.Amounts[j] {
				return fmt.Errorf("source cell differs on row %d for %s", src.Number, name)
			}
			if src.IsSession && (c.ChargeCents == nil || *c.ChargeCents != charges[j] || c.PaidCents != paid[j]) {
				return fmt.Errorf("charge differs on row %d for %s", src.Number, name)
			}
			if !src.IsSession && (c.ChargeCents != nil || c.PaidCents != 0) {
				return fmt.Errorf("unexpected session share")
			}
			if src.Amounts[j] != 0 {
				expected = append(expected, Movement{AccountID: p.AccountID, AmountCents: src.Amounts[j]})
			}
			report.VerifiedCells++
		}
		if r.ClubCents != 0 {
			expected = append(expected, Movement{AccountID: surplus, AmountCents: r.ClubCents})
		}
		if len(expected) == 0 {
			expected = append(expected, Movement{AccountID: surplus})
		}
		if err := verifyMovements(tx, r.TransactionID, expected); err != nil {
			return err
		}
		if src.IsSession && basis == "recorded" {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Row %d: no title date; uses recorded date %s", src.Number, src.Date.Format("2006-01-02")))
		}
	}
	for i, name := range source.Names {
		if i == source.ClubIndex {
			continue
		}
		p := byName[name]
		member, active := members[name]
		if p.Account == nil || p.Account.Kind != models.AccountKindPlayer || (p.UserID != nil) != active || (p.Account.UserID != nil) != active {
			return fmt.Errorf("participant identity differs for %s", name)
		}
		if active && (p.User == nil || p.User.Email != member.Email || *p.Account.UserID != *p.UserID) {
			return fmt.Errorf("user identity differs for %s", name)
		}
		var actual int64
		if err := tx.Raw(`SELECT COALESCE(SUM(e.amount_cents),0) FROM ledger_entries e JOIN splitwise_records r ON r.transaction_id=e.transaction_id WHERE r.import_id=? AND e.account_id=?`, batch.ID, p.AccountID).Scan(&actual).Error; err != nil {
			return err
		}
		if actual != source.Totals[i] || p.ClosingCents != source.Totals[i] {
			return fmt.Errorf("closing balance differs for %s", name)
		}
		report.Balances = append(report.Balances, ImportBalance{Name: name, Inactive: !active, ExpectedCents: source.Totals[i], ActualCents: actual})
	}
	report.Transactions = len(records)
	report.ResidualCents, err = clubPositionResidual(tx)
	if err != nil {
		return err
	}
	if report.ResidualCents != 0 {
		return ErrInvariantViolated(report.ResidualCents)
	}
	return nil
}
