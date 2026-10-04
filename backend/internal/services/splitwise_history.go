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

func importedSessionView(id uuid.UUID) (*SettlementView, error) {
	var record models.SplitwiseRecord
	err := database.DB.Where("id = ? AND is_session = true", id).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
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
