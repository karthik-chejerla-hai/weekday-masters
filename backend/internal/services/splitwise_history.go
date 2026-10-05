package services

import (
	"errors"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
)

type ImportedSessionLine struct {
	Name        string     `json:"name"`
	UserID      *uuid.UUID `json:"user_id,omitempty"`
	Inactive    bool       `json:"inactive"`
	ChargeCents int64      `json:"charge_cents"`
	PaidCents   int64      `json:"paid_cents"`
	NetCents    int64      `json:"net_cents"`
}

type ImportedSessionSource struct {
	Description  string `json:"description"`
	RecordedDate string `json:"recorded_date"`
	CostCents    int64  `json:"cost_cents"`
}

type ImportedSessionView struct {
	PlayedDate string                  `json:"played_date"`
	DateBasis  string                  `json:"date_basis"`
	TotalCents int64                   `json:"total_cents"`
	Lines      []ImportedSessionLine   `json:"lines"`
	Sources    []ImportedSessionSource `json:"sources"`
}

// A schedule with no native settlement on an imported play date is already
// represented by the imported game. Keep native settlement history (including
// reversals) distinct. Both history and settlement guards use this projection.
const importedSessionsQuery = `WITH imported AS (
 SELECT import_id, played_date,
        (array_agg(id ORDER BY row_number))[1] AS session_id,
        (array_agg(description ORDER BY row_number))[1] AS title,
        (array_agg(date_basis ORDER BY row_number))[1] AS date_basis,
        SUM(cost_cents) AS total_cents
 FROM splitwise_records WHERE is_session = true
 GROUP BY import_id, played_date
), imported_schedule AS (
 SELECT s.id AS session_id, i.session_id AS imported_session_id
 FROM sessions s JOIN imported i ON i.played_date = s.session_date
 WHERE NOT EXISTS (SELECT 1 FROM settlements st WHERE st.session_id = s.id)
)`

// scheduledSessionsWithSettlementStatus keeps scheduled-session lookups in step
// with history. Imported play dates count as settled only when the schedule has
// no native settlement history; a reversed native settlement remains unpaid.
func scheduledSessionsWithSettlementStatus(db *gorm.DB) *gorm.DB {
	return db.Table("(?) AS s", db.Raw(importedSessionsQuery+`
 SELECT s.*,
   (EXISTS (SELECT 1 FROM settlements st WHERE st.session_id = s.id AND st.reversed_at IS NULL)
    OR EXISTS (SELECT 1 FROM imported_schedule i WHERE i.session_id = s.id)) AS settled
 FROM sessions s`))
}

// importedSessionRecordID accepts either an imported source ID or an old
// schedule ID. A zero UUID means that the session belongs to the native flow.
func importedSessionRecordID(db *gorm.DB, id uuid.UUID) (uuid.UUID, error) {
	var match struct{ ID uuid.UUID }
	err := db.Raw(importedSessionsQuery+`
 SELECT id FROM splitwise_records WHERE id = ? AND is_session = true
 UNION ALL
 SELECT imported_session_id AS id FROM imported_schedule WHERE session_id = ?
 ORDER BY id LIMIT 1`, id, id).Scan(&match).Error
	return match.ID, err
}

func rejectImportedSettlement(db *gorm.DB, id uuid.UUID) error {
	importedID, err := importedSessionRecordID(db, id)
	if err != nil {
		return err
	}
	if importedID != uuid.Nil {
		return ErrNotSettleable("This session was settled in Splitwise. View its imported split instead.")
	}
	return nil
}

func importedSessionView(id uuid.UUID) (*SettlementView, error) {
	importedID, err := importedSessionRecordID(database.DB, id)
	if err != nil || importedID == uuid.Nil {
		return nil, err
	}
	var record models.SplitwiseRecord
	if err := database.DB.First(&record, "id = ?", importedID).Error; err != nil {
		return nil, err
	}
	// Extra-hour rows are separate source transactions but part of the same
	// night's session. Keep every source row visible in the breakdown.
	var rows []models.SplitwiseRecord
	if err := database.DB.Preload("Changes.Participant.User").Where("import_id = ? AND played_date = ? AND is_session = true", record.ImportID, record.PlayedDate).Order("row_number").Find(&rows).Error; err != nil {
		return nil, err
	}
	imported := &ImportedSessionView{PlayedDate: record.PlayedDate.Format("2006-01-02"), DateBasis: record.DateBasis, Lines: []ImportedSessionLine{}}
	byParticipant := map[uuid.UUID]*ImportedSessionLine{}
	for _, row := range rows {
		imported.TotalCents += row.CostCents
		imported.Sources = append(imported.Sources, ImportedSessionSource{Description: row.Description, RecordedDate: row.RecordedDate.Format("2006-01-02"), CostCents: row.CostCents})
		for _, change := range row.Changes {
			if change.ChargeCents == nil || (*change.ChargeCents == 0 && change.PaidCents == 0) {
				continue
			}
			p := change.Participant
			if p == nil {
				return nil, errors.New("imported participant missing")
			}
			line := byParticipant[p.ID]
			if line == nil {
				name := strings.TrimSuffix(p.SourceName, " (removed)")
				if p.User != nil {
					name = p.User.DisplayName()
				}
				line = &ImportedSessionLine{Name: name, UserID: p.UserID, Inactive: p.UserID == nil}
				byParticipant[p.ID] = line
			}
			line.ChargeCents += *change.ChargeCents
			line.PaidCents += change.PaidCents
			line.NetCents += change.NetCents
		}
	}
	for _, line := range byParticipant {
		imported.Lines = append(imported.Lines, *line)
	}
	sort.Slice(imported.Lines, func(i, j int) bool { return imported.Lines[i].Name < imported.Lines[j].Name })
	return &SettlementView{Session: SessionSummary{ID: rows[0].ID, Title: rows[0].Description}, Imported: imported}, nil
}
