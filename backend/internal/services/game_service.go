package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const GamePageSize = 50

type GameService struct{}

func NewGameService() *GameService { return &GameService{} }

type GameInput struct {
	TeamA     []uuid.UUID `json:"team_a"`
	TeamB     []uuid.UUID `json:"team_b"`
	ScoreA    *int        `json:"score_a"`
	ScoreB    *int        `json:"score_b"`
	RequestID uuid.UUID   `json:"request_id"`
	Version   int         `json:"version"`
}
type GamePlayer struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	FullName string    `json:"full_name,omitempty"`
}
type GameView struct {
	models.GameResult
	TeamA        []GamePlayer `json:"team_a"`
	TeamB        []GamePlayer `json:"team_b"`
	SessionTitle string       `json:"session_title"`
	SessionDate  string       `json:"session_date"`
	RecorderName string       `json:"recorder_name"`
	EditorName   string       `json:"editor_name"`
}
type GamePreview struct {
	Session SessionSummary `json:"session"`
	TeamA   []GamePlayer   `json:"team_a"`
	TeamB   []GamePlayer   `json:"team_b"`
	ScoreA  int            `json:"score_a"`
	ScoreB  int            `json:"score_b"`
}
type GameList struct {
	Items []GameView `json:"items"`
	Total int64      `json:"total"`
}
type GameComparison struct {
	GameList
	WinsA   int64 `json:"wins_a"`
	WinsB   int64 `json:"wins_b"`
	PointsA int64 `json:"points_a"`
	PointsB int64 `json:"points_b"`
}

func gameInvalid(message string) error  { return newLedgerError("invalid_game", 422, message) }
func gameConflict(message string) error { return newLedgerError("game_conflict", 409, message) }
func gameActor(actor *models.User) error {
	if actor == nil || !actor.IsApproved() {
		return newLedgerError("forbidden", 403, "Approved membership is required.")
	}
	return nil
}
func normalizedTeams(a, b []uuid.UUID, size int) ([]uuid.UUID, []uuid.UUID, error) {
	if len(a) != size || len(b) != size {
		return nil, nil, gameInvalid("Select two different players for each team.")
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range append(append([]uuid.UUID{}, a...), b...) {
		if id == uuid.Nil || seen[id] {
			return nil, nil, gameInvalid("Each player must appear exactly once.")
		}
		seen[id] = true
	}
	a = append([]uuid.UUID{}, a...)
	b = append([]uuid.UUID{}, b...)
	sort.Slice(a, func(i, j int) bool { return a[i].String() < a[j].String() })
	sort.Slice(b, func(i, j int) bool { return b[i].String() < b[j].String() })
	return a, b, nil
}
func normalizeGame(in GameInput) (GameInput, error) {
	a, b, err := normalizedTeams(in.TeamA, in.TeamB, 2)
	if err != nil {
		return in, err
	}
	in.TeamA = a
	in.TeamB = b
	if in.ScoreA == nil || in.ScoreB == nil || *in.ScoreA < 0 || *in.ScoreB < 0 || *in.ScoreA > 99 || *in.ScoreB > 99 || *in.ScoreA == *in.ScoreB {
		return in, gameInvalid("Enter two different whole-number scores from 0 to 99.")
	}
	return in, nil
}
func gameMembers(tx *gorm.DB, in GameInput, existing *models.GameResult) error {
	ids := append(append([]uuid.UUID{}, in.TeamA...), in.TeamB...)
	var users []models.User
	if err := tx.Where("id IN ?", ids).Find(&users).Error; err != nil {
		return err
	}
	if len(users) != 4 {
		return gameInvalid("Choose four club members.")
	}
	old := map[uuid.UUID]bool{}
	if existing != nil {
		for _, id := range []uuid.UUID{existing.TeamA1, existing.TeamA2, existing.TeamB1, existing.TeamB2} {
			old[id] = true
		}
	}
	for _, u := range users {
		if !u.IsApproved() && !old[u.ID] {
			return gameInvalid("New players must be approved club members.")
		}
	}
	return nil
}
func playableSession(tx *gorm.DB, id uuid.UUID, lock bool) (*models.Session, error) {
	var session models.Session
	if lock {
		tx = tx.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := tx.First(&session, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if session.Status == models.SessionStatusCancelled || session.StartsAt == nil || session.StartsAt.After(time.Now()) {
		return nil, gameInvalid("Record games only after a session starts. Cancelled sessions cannot receive games.")
	}
	return &session, nil
}
func assignGame(g *models.GameResult, in GameInput) {
	g.TeamA1 = in.TeamA[0]
	g.TeamA2 = in.TeamA[1]
	g.TeamB1 = in.TeamB[0]
	g.TeamB2 = in.TeamB[1]
	g.ScoreA = *in.ScoreA
	g.ScoreB = *in.ScoreB
}
func gameQuery(tx *gorm.DB) *gorm.DB {
	return tx.Preload("Session").Preload("A1").Preload("A2").Preload("B1").Preload("B2").Preload("Recorder").Preload("Editor")
}
func gameView(g models.GameResult) GameView {
	player := func(u *models.User) GamePlayer { return GamePlayer{ID: u.ID, Name: u.DisplayName(), FullName: u.Name} }
	return GameView{GameResult: g, TeamA: []GamePlayer{player(g.A1), player(g.A2)}, TeamB: []GamePlayer{player(g.B1), player(g.B2)}, SessionTitle: g.Session.Title, SessionDate: g.Session.SessionDate.Format("2006-01-02"), RecorderName: g.Recorder.DisplayName(), EditorName: g.Editor.DisplayName()}
}
func loadGame(tx *gorm.DB, id uuid.UUID) (*GameView, error) {
	var g models.GameResult
	if err := gameQuery(tx).First(&g, "id = ?", id).Error; err != nil {
		return nil, err
	}
	v := gameView(g)
	return &v, nil
}
func appendGameRevision(tx *gorm.DB, g *models.GameResult) (*GameView, error) {
	view, err := loadGame(tx, g.ID)
	if err != nil {
		return nil, err
	}
	snapshot, err := json.Marshal(view)
	if err != nil {
		return nil, err
	}
	err = tx.Create(&models.GameRevision{GameID: g.ID, Version: g.Version, Snapshot: string(snapshot), ChangedBy: g.UpdatedBy}).Error
	return view, err
}
func (s *GameService) Preview(ctx context.Context, sessionID uuid.UUID, in GameInput, actor *models.User) (*GamePreview, error) {
	if err := gameActor(actor); err != nil {
		return nil, err
	}
	in, err := normalizeGame(in)
	if err != nil {
		return nil, err
	}
	tx := database.DB.WithContext(ctx)
	session, err := playableSession(tx, sessionID, false)
	if err != nil {
		return nil, err
	}
	if err = gameMembers(tx, in, nil); err != nil {
		return nil, err
	}
	players, err := s.Players(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[uuid.UUID]GamePlayer{}
	for _, p := range players {
		byID[p.ID] = p
	}
	return &GamePreview{Session: SessionSummary{ID: session.ID, Title: session.Title, StartsAt: session.StartsAt, EndsAt: session.EndsAt}, TeamA: []GamePlayer{byID[in.TeamA[0]], byID[in.TeamA[1]]}, TeamB: []GamePlayer{byID[in.TeamB[0]], byID[in.TeamB[1]]}, ScoreA: *in.ScoreA, ScoreB: *in.ScoreB}, nil
}
func (s *GameService) Create(ctx context.Context, sessionID uuid.UUID, in GameInput, actor *models.User) (*GameView, error) {
	if err := gameActor(actor); err != nil {
		return nil, err
	}
	in, err := normalizeGame(in)
	if err != nil {
		return nil, err
	}
	if in.RequestID == uuid.Nil {
		return nil, gameInvalid("A request ID is required to save a game.")
	}
	raw, _ := json.Marshal(struct {
		Session        uuid.UUID
		A, B           []uuid.UUID
		ScoreA, ScoreB int
	}{sessionID, in.TeamA, in.TeamB, *in.ScoreA, *in.ScoreB})
	hash := fmt.Sprintf("%x", sha256.Sum256(raw))
	var view *GameView
	err = database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var prior models.GameResult
		found := tx.Where("created_by = ? AND request_id = ?", actor.ID, in.RequestID).First(&prior).Error
		if found == nil {
			if prior.RequestHash != hash {
				return gameConflict("This request ID was already used for a different result.")
			}
			var e error
			view, e = loadGame(tx, prior.ID)
			return e
		}
		if !errors.Is(found, gorm.ErrRecordNotFound) {
			return found
		}
		if _, err := playableSession(tx, sessionID, true); err != nil {
			return err
		}
		if err := gameMembers(tx, in, nil); err != nil {
			return err
		}
		g := models.GameResult{SessionID: sessionID, CreatedBy: actor.ID, UpdatedBy: actor.ID, RequestID: in.RequestID, RequestHash: hash, Version: 1}
		assignGame(&g, in)
		result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "created_by"}, {Name: "request_id"}}, DoNothing: true}).Create(&g)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			if err := tx.Where("created_by = ? AND request_id = ?", actor.ID, in.RequestID).First(&prior).Error; err != nil {
				return err
			}
			if prior.RequestHash != hash {
				return gameConflict("This request ID was already used for a different result.")
			}
			var e error
			view, e = loadGame(tx, prior.ID)
			return e
		}
		var e error
		view, e = appendGameRevision(tx, &g)
		return e
	})
	return view, err
}
func (s *GameService) Update(ctx context.Context, id uuid.UUID, in GameInput, actor *models.User) (*GameView, error) {
	in, err := normalizeGame(in)
	if err != nil {
		return nil, err
	}
	return s.change(ctx, id, in, in.Version, false, actor)
}
func (s *GameService) Void(ctx context.Context, id uuid.UUID, version int, actor *models.User) (*GameView, error) {
	return s.change(ctx, id, GameInput{}, version, true, actor)
}
func (s *GameService) change(ctx context.Context, id uuid.UUID, in GameInput, version int, void bool, actor *models.User) (*GameView, error) {
	if err := gameActor(actor); err != nil {
		return nil, err
	}
	var view *GameView
	err := database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var g models.GameResult
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&g, "id = ?", id).Error; err != nil {
			return err
		}
		if g.CreatedBy != actor.ID && actor.Role != models.RoleAdmin {
			return newLedgerError("forbidden", 403, "Only the recorder or an admin can change this result.")
		}
		if g.Version != version {
			return gameConflict("This result changed. Reload it before making another change.")
		}
		if g.VoidedAt != nil {
			return gameConflict("This result has been voided. Record a new game if needed.")
		}
		if void {
			now := time.Now()
			g.VoidedAt = &now
		} else {
			if err := gameMembers(tx, in, &g); err != nil {
				return err
			}
			assignGame(&g, in)
		}
		g.Version++
		g.UpdatedBy = actor.ID
		if err := tx.Save(&g).Error; err != nil {
			return err
		}
		var e error
		view, e = appendGameRevision(tx, &g)
		return e
	})
	return view, err
}
func gamePage(tx *gorm.DB, offset int) ([]GameView, error) {
	var rows []models.GameResult
	if err := gameQuery(tx).Order("game_results.created_at DESC, game_results.id DESC").Limit(GamePageSize).Offset(offset).Find(&rows).Error; err != nil {
		return nil, err
	}
	views := make([]GameView, 0, len(rows))
	for _, row := range rows {
		views = append(views, gameView(row))
	}
	return views, nil
}
func (s *GameService) List(ctx context.Context, sessionID uuid.UUID, offset int) (*GameList, error) {
	if offset < 0 {
		return nil, gameInvalid("Page offset must be zero or more.")
	}
	result := &GameList{Items: []GameView{}}
	err := database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session models.Session
		if err := tx.First(&session, "id = ?", sessionID).Error; err != nil {
			return err
		}
		q := tx.Model(&models.GameResult{}).Where("session_id = ?", sessionID)
		if err := q.Count(&result.Total).Error; err != nil {
			return err
		}
		var err error
		result.Items, err = gamePage(q, offset)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}
func (s *GameService) Revisions(ctx context.Context, id uuid.UUID) ([]GameView, error) {
	var game models.GameResult
	if err := database.DB.WithContext(ctx).First(&game, "id = ?", id).Error; err != nil {
		return nil, err
	}
	var rows []models.GameRevision
	if err := database.DB.WithContext(ctx).Where("game_id = ?", id).Order("version").Find(&rows).Error; err != nil {
		return nil, err
	}
	views := make([]GameView, 0, len(rows))
	for _, r := range rows {
		var v GameView
		if err := json.Unmarshal([]byte(r.Snapshot), &v); err != nil {
			return nil, err
		}
		views = append(views, v)
	}
	return views, nil
}
func (s *GameService) Players(ctx context.Context) ([]GamePlayer, error) {
	var users []models.User
	err := database.DB.WithContext(ctx).Where(`membership_status = ? OR id IN (SELECT team_a1 FROM game_results UNION SELECT team_a2 FROM game_results UNION SELECT team_b1 FROM game_results UNION SELECT team_b2 FROM game_results)`, models.MembershipApproved).Order("name,id").Find(&users).Error
	players := make([]GamePlayer, 0, len(users))
	for _, u := range users {
		players = append(players, GamePlayer{ID: u.ID, Name: u.DisplayName(), FullName: u.Name})
	}
	return players, err
}
func (s *GameService) HeadToHead(ctx context.Context, a, b []uuid.UUID, offset int) (*GameComparison, error) {
	if (len(a) != 1 && len(a) != 2) || offset < 0 {
		return nil, gameInvalid("Choose one opponent per side or two players per team.")
	}
	a, b, err := normalizedTeams(a, b, len(a))
	if err != nil {
		return nil, err
	}
	// The forward expression identifies the selected A side. Reverse swaps sides,
	// so wins and points always follow the caller's selected opponents.
	var forward, reverse string
	var fargs, rargs []any
	if len(a) == 1 {
		forward = "(team_a1 = ? OR team_a2 = ?) AND (team_b1 = ? OR team_b2 = ?)"
		reverse = forward
		fargs = []any{a[0], a[0], b[0], b[0]}
		rargs = []any{b[0], b[0], a[0], a[0]}
	} else {
		forward = "team_a1 = ? AND team_a2 = ? AND team_b1 = ? AND team_b2 = ?"
		reverse = forward
		fargs = []any{a[0], a[1], b[0], b[1]}
		rargs = []any{b[0], b[1], a[0], a[1]}
	}
	result := &GameComparison{GameList: GameList{Items: []GameView{}}}
	err = database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		where := "voided_at IS NULL AND ((" + forward + ") OR (" + reverse + "))"
		args := append(append([]any{}, fargs...), rargs...)
		oriented := tx.Model(&models.GameResult{}).Select("CASE WHEN ("+forward+") THEN score_a ELSE score_b END AS a, CASE WHEN ("+forward+") THEN score_b ELSE score_a END AS b", append(append([]any{}, fargs...), fargs...)...).Where(where, args...)
		var totals struct{ Total, WinsA, WinsB, PointsA, PointsB int64 }
		if err := tx.Table("(?) AS scores", oriented).Select("COUNT(*) AS total, COUNT(*) FILTER (WHERE a > b) AS wins_a, COUNT(*) FILTER (WHERE b > a) AS wins_b, COALESCE(SUM(a),0) AS points_a, COALESCE(SUM(b),0) AS points_b").Scan(&totals).Error; err != nil {
			return err
		}
		result.Total = totals.Total
		result.WinsA = totals.WinsA
		result.WinsB = totals.WinsB
		result.PointsA = totals.PointsA
		result.PointsB = totals.PointsB
		var err error
		result.Items, err = gamePage(tx.Model(&models.GameResult{}).Where(where, args...), offset)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}
