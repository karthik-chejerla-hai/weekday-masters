package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Import records retain source facts separately from live settlement rates.
// A historical participant can own ledger history without having a user/login.
type SplitwiseImport struct {
	ID                 uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	SourceHash         string     `gorm:"size:64;not null;uniqueIndex" json:"source_hash"`
	MappingHash        string     `gorm:"size:64;not null" json:"mapping_hash"`
	Cutoff             time.Time  `gorm:"type:date;not null" json:"cutoff"`
	AssetsConfirmed    bool       `json:"assets_confirmed"`
	AssetTransactionID *uuid.UUID `gorm:"type:uuid" json:"asset_transaction_id,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

type SplitwiseParticipant struct {
	ID           uuid.UUID        `gorm:"type:uuid;primaryKey"`
	ImportID     uuid.UUID        `gorm:"type:uuid;not null;uniqueIndex:idx_import_participant"`
	SourceName   string           `gorm:"not null;uniqueIndex:idx_import_participant"`
	UserID       *uuid.UUID       `gorm:"type:uuid"`
	AccountID    uuid.UUID        `gorm:"type:uuid;not null"`
	ClosingCents int64            `gorm:"not null"`
	Account      *Account         `gorm:"foreignKey:AccountID"`
	User         *User            `gorm:"foreignKey:UserID"`
	Import       *SplitwiseImport `gorm:"foreignKey:ImportID"`
}

// A session import has its own identity, with no invented scheduling metadata.
// Its ID is used by the history/breakdown API and the ledger's session link.
type SplitwiseRecord struct {
	ID            uuid.UUID         `gorm:"type:uuid;primaryKey"`
	ImportID      uuid.UUID         `gorm:"type:uuid;not null;uniqueIndex:idx_import_record"`
	RowNumber     int               `gorm:"not null;uniqueIndex:idx_import_record"`
	RecordedDate  time.Time         `gorm:"type:date;not null"`
	PlayedDate    time.Time         `gorm:"type:date;not null"`
	DateBasis     string            `gorm:"not null"`
	Description   string            `gorm:"not null"`
	Category      string            `gorm:"not null"`
	CostCents     int64             `gorm:"not null"`
	ClubCents     int64             `gorm:"not null"`
	IsSession     bool              `gorm:"not null"`
	TransactionID uuid.UUID         `gorm:"type:uuid;not null;uniqueIndex"`
	Import        *SplitwiseImport  `gorm:"foreignKey:ImportID"`
	Transaction   *Transaction      `gorm:"foreignKey:TransactionID"`
	Changes       []SplitwiseChange `gorm:"foreignKey:RecordID"`
}

type SplitwiseChange struct {
	ID            uuid.UUID             `gorm:"type:uuid;primaryKey"`
	RecordID      uuid.UUID             `gorm:"type:uuid;not null;uniqueIndex:idx_import_change"`
	ParticipantID uuid.UUID             `gorm:"type:uuid;not null;uniqueIndex:idx_import_change"`
	NetCents      int64                 `gorm:"not null"`
	ChargeCents   *int64                // Exact expense share, only for reviewed session rows.
	PaidCents     int64                 `gorm:"not null"`
	Participant   *SplitwiseParticipant `gorm:"foreignKey:ParticipantID"`
}

func (b *SplitwiseImport) BeforeCreate(*gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}
func (p *SplitwiseParticipant) BeforeCreate(*gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
func (r *SplitwiseRecord) BeforeCreate(*gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}
func (c *SplitwiseChange) BeforeCreate(*gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}
