package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

// GameResult stores one doubles game. Pair order is canonical within each side.
type GameResult struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	SessionID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"session_id"`
	TeamA1      uuid.UUID  `gorm:"type:uuid;not null;index;check:game_distinct_players,team_a1 <> team_a2 AND team_a1 <> team_b1 AND team_a1 <> team_b2 AND team_a2 <> team_b1 AND team_a2 <> team_b2 AND team_b1 <> team_b2" json:"-"`
	TeamA2      uuid.UUID  `gorm:"type:uuid;not null;index" json:"-"`
	TeamB1      uuid.UUID  `gorm:"type:uuid;not null;index" json:"-"`
	TeamB2      uuid.UUID  `gorm:"type:uuid;not null;index" json:"-"`
	ScoreA      int        `gorm:"not null;check:game_scores,score_a BETWEEN 0 AND 99 AND score_b BETWEEN 0 AND 99 AND score_a <> score_b" json:"score_a"`
	ScoreB      int        `gorm:"not null" json:"score_b"`
	CreatedBy   uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_game_request" json:"created_by"`
	UpdatedBy   uuid.UUID  `gorm:"type:uuid;not null" json:"updated_by"`
	RequestID   uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_game_request" json:"-"`
	RequestHash string     `gorm:"size:64;not null" json:"-"`
	Version     int        `gorm:"not null;check:game_version,version > 0" json:"version"`
	VoidedAt    *time.Time `json:"voided_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Session     *Session   `gorm:"foreignKey:SessionID" json:"-"`
	A1          *User      `gorm:"foreignKey:TeamA1" json:"-"`
	A2          *User      `gorm:"foreignKey:TeamA2" json:"-"`
	B1          *User      `gorm:"foreignKey:TeamB1" json:"-"`
	B2          *User      `gorm:"foreignKey:TeamB2" json:"-"`
	Recorder    *User      `gorm:"foreignKey:CreatedBy" json:"-"`
	Editor      *User      `gorm:"foreignKey:UpdatedBy" json:"-"`
}

func (g *GameResult) BeforeCreate(tx *gorm.DB) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	return nil
}

// Each accepted write appends one complete public snapshot. Revisions are immutable.
type GameRevision struct {
	ID        uuid.UUID   `gorm:"type:uuid;primaryKey" json:"id"`
	GameID    uuid.UUID   `gorm:"type:uuid;not null;uniqueIndex:idx_game_revision" json:"game_id"`
	Version   int         `gorm:"not null;uniqueIndex:idx_game_revision" json:"version"`
	Snapshot  string      `gorm:"type:jsonb;not null" json:"-"`
	ChangedBy uuid.UUID   `gorm:"type:uuid;not null" json:"changed_by"`
	CreatedAt time.Time   `json:"created_at"`
	Game      *GameResult `gorm:"foreignKey:GameID" json:"-"`
	Actor     *User       `gorm:"foreignKey:ChangedBy" json:"-"`
}

func (g *GameRevision) BeforeCreate(tx *gorm.DB) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	return nil
}
