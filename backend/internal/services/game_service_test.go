package services

import (
	"context"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"sync"
	"testing"
	"time"
)

func gameFixture(t *testing.T) (*GameService, *models.Session, []*models.User, GameInput) {
	t.Helper()
	requireDB(t)
	players := []*models.User{}
	for _, name := range []string{"Alice One", "Bob Two", "Cara Three", "Dan Four", "Eve Five"} {
		u := &models.User{Auth0ID: uuid.NewString(), Email: uuid.NewString() + "@test.invalid", Name: name, Role: models.RolePlayer, MembershipStatus: models.MembershipApproved}
		if err := database.DB.Create(u).Error; err != nil {
			t.Fatal(err)
		}
		players = append(players, u)
	}
	session := &models.Session{Title: "Doubles", SessionDate: time.Now().AddDate(0, 0, -1), StartTime: "18:00", EndTime: "20:00", Courts: 1, Status: models.SessionStatusOpen, CreatedBy: players[0].ID}
	if err := database.DB.Create(session).Error; err != nil {
		t.Fatal(err)
	}
	a, b := 21, 17
	return NewGameService(), session, players, GameInput{TeamA: []uuid.UUID{players[0].ID, players[1].ID}, TeamB: []uuid.UUID{players[2].ID, players[3].ID}, ScoreA: &a, ScoreB: &b, RequestID: uuid.New()}
}
func TestGameCreateRetriesAndValidation(t *testing.T) {
	s, se, p, in := gameFixture(t)
	ctx := context.Background()
	preview, err := s.Preview(ctx, se.ID, in, p[4])
	if err != nil || len(preview.TeamA) != 2 {
		t.Fatalf("preview %v %v", preview, err)
	}
	var count int64
	database.DB.Model(&models.GameResult{}).Count(&count)
	if count != 0 {
		t.Fatal("preview wrote a game")
	}
	game, err := s.Create(ctx, se.ID, in, p[4])
	if err != nil {
		t.Fatal(err)
	}
	retry, err := s.Create(ctx, se.ID, in, p[4])
	if err != nil || retry.ID != game.ID {
		t.Fatalf("retry %v %v", retry, err)
	}
	in.TeamA[0], in.TeamA[1] = in.TeamA[1], in.TeamA[0]
	if _, err = s.Create(ctx, se.ID, in, p[4]); err != nil {
		t.Fatal("partner order must be idempotent", err)
	}
	score := 22
	in.ScoreA = &score
	if _, err = s.Create(ctx, se.ID, in, p[4]); err == nil {
		t.Fatal("changed retry accepted")
	}
	for _, test := range []struct {
		name   string
		change func(*GameInput)
	}{
		{"missing score", func(i *GameInput) { i.ScoreA = nil }},
		{"tie", func(i *GameInput) { i.ScoreA = i.ScoreB }},
		{"negative", func(i *GameInput) { n := -1; i.ScoreA = &n }},
		{"too high", func(i *GameInput) { n := 100; i.ScoreA = &n }},
		{"repeated player", func(i *GameInput) { i.TeamB = []uuid.UUID{i.TeamA[0], i.TeamB[1]} }},
		{"wrong size", func(i *GameInput) { i.TeamA = i.TeamA[:1] }},
		{"unknown player", func(i *GameInput) { i.TeamB = []uuid.UUID{uuid.New(), i.TeamB[1]} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			bad := in
			bad.RequestID = uuid.New()
			test.change(&bad)
			if _, err := s.Create(ctx, se.ID, bad, p[4]); err == nil {
				t.Fatal("accepted invalid game")
			}
		})
	}
	in.RequestID = uuid.New()
	if _, err = s.Create(ctx, se.ID, in, p[4]); err != nil {
		t.Fatal(err)
	}
	list, err := s.List(ctx, se.ID, 0)
	if err != nil || list.Total != 2 {
		t.Fatalf("list %+v %v", list, err)
	}
}
func TestGameConcurrentCreateAndCorrections(t *testing.T) {
	s, se, p, in := gameFixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.Create(ctx, se.ID, in, p[4]); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	list, _ := s.List(ctx, se.ID, 0)
	if list.Total != 1 {
		t.Fatalf("created %d", list.Total)
	}
	g := list.Items[0]
	in.Version = g.Version
	score := 23
	in.ScoreA = &score
	if _, err := s.Update(ctx, g.ID, in, p[0]); err == nil {
		t.Fatal("other member corrected")
	}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.Update(ctx, g.ID, in, p[4]); results <- err }()
	}
	wg.Wait()
	close(results)
	ok, conflict := 0, 0
	for err := range results {
		if err == nil {
			ok++
		} else if e, yes := AsLedgerError(err); yes && e.Status == 409 {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("updates %d conflicts %d", ok, conflict)
	}
	audit, err := s.Revisions(ctx, g.ID)
	if err != nil || len(audit) != 2 || audit[0].ScoreA != 21 || audit[1].ScoreA != 23 {
		t.Fatalf("audit %+v %v", audit, err)
	}
	p[0].Role = models.RoleAdmin
	if _, err = s.Void(ctx, g.ID, 2, p[0]); err != nil {
		t.Fatal(err)
	}
	audit, _ = s.Revisions(ctx, g.ID)
	if len(audit) != 3 || audit[2].VoidedAt == nil {
		t.Fatal("missing void audit")
	}
	if _, err = s.Update(ctx, g.ID, in, p[4]); err == nil {
		t.Fatal("changed voided game")
	}
}
func TestGameHeadToHead(t *testing.T) {
	s, se, p, in := gameFixture(t)
	ctx := context.Background()
	first, err := s.Create(ctx, se.ID, in, p[4])
	if err != nil {
		t.Fatal(err)
	}
	in.RequestID = uuid.New()
	in.TeamA, in.TeamB = in.TeamB, in.TeamA
	if _, err = s.Create(ctx, se.ID, in, p[4]); err != nil {
		t.Fatal(err)
	}
	in.RequestID = uuid.New()
	in.TeamA = []uuid.UUID{p[0].ID, p[2].ID}
	in.TeamB = []uuid.UUID{p[1].ID, p[3].ID}
	if _, err = s.Create(ctx, se.ID, in, p[4]); err != nil {
		t.Fatal(err)
	}
	h, err := s.HeadToHead(ctx, []uuid.UUID{p[0].ID}, []uuid.UUID{p[2].ID}, 0)
	if err != nil || h.Total != 2 || h.WinsA != 1 || h.WinsB != 1 || h.PointsA != 38 || h.PointsB != 38 {
		t.Fatalf("h2h %+v %v", h, err)
	}
	team, err := s.HeadToHead(ctx, []uuid.UUID{p[1].ID, p[0].ID}, []uuid.UUID{p[3].ID, p[2].ID}, 50)
	if err != nil || team.Total != 2 || len(team.Items) != 0 {
		t.Fatalf("team %+v %v", team, err)
	}
	if _, err = s.Void(ctx, first.ID, 1, p[4]); err != nil {
		t.Fatal(err)
	}
	h, _ = s.HeadToHead(ctx, []uuid.UUID{p[0].ID}, []uuid.UUID{p[2].ID}, 0)
	if h.Total != 1 || h.WinsB != 1 {
		t.Fatalf("void counted %+v", h)
	}
	database.DB.Model(p[0]).Update("membership_status", models.MembershipRemoved)
	players, err := s.Players(ctx)
	if err != nil || len(players) != 5 {
		t.Fatalf("removed history %+v %v", players, err)
	}
	if err := NewSessionService().DeleteSession(se.ID); err == nil {
		t.Fatal("deleted scored session")
	}
}
func TestGameSessionAndMembershipRules(t *testing.T) {
	s, se, p, in := gameFixture(t)
	ctx := context.Background()
	p[4].MembershipStatus = models.MembershipPending
	if _, err := s.Create(ctx, se.ID, in, p[4]); err == nil {
		t.Fatal("pending recorder")
	}
	p[4].MembershipStatus = models.MembershipApproved
	database.DB.Model(p[0]).Update("membership_status", models.MembershipRemoved)
	if _, err := s.Create(ctx, se.ID, in, p[4]); err == nil {
		t.Fatal("removed player")
	}
	database.DB.Model(p[0]).Update("membership_status", models.MembershipApproved)
	database.DB.Model(se).Update("status", models.SessionStatusCancelled)
	if _, err := s.Create(ctx, se.ID, in, p[4]); err == nil {
		t.Fatal("cancelled session")
	}
	database.DB.Model(se).Update("status", models.SessionStatusOpen)
	future := time.Now().Add(time.Hour)
	database.DB.Model(se).UpdateColumn("starts_at", future)
	if _, err := s.Create(ctx, se.ID, in, p[4]); err == nil {
		t.Fatal("future session")
	}
}

func TestGameSessionDeletionPreservesResults(t *testing.T) {
	for _, state := range []string{"active", "voided"} {
		t.Run(state, func(t *testing.T) {
			s, session, players, input := gameFixture(t)
			ctx := context.Background()
			game, err := s.Create(ctx, session.ID, input, players[4])
			if err != nil {
				t.Fatal(err)
			}
			versions := 1
			if state == "voided" {
				if _, err := s.Void(ctx, game.ID, game.Version, players[4]); err != nil {
					t.Fatal(err)
				}
				versions++
			}
			sessions := NewSessionService()
			err = sessions.DeleteSession(session.ID)
			if e, ok := AsLedgerError(err); !ok || e.Code != "game_conflict" || e.Status != 409 {
				t.Fatalf("expected an explicit game conflict, got %v", err)
			}
			stored, err := sessions.GetSessionByID(session.ID)
			if err != nil || stored.Status != models.SessionStatusOpen {
				t.Fatalf("deletion changed the session: %+v %v", stored, err)
			}
			history, err := s.Revisions(ctx, game.ID)
			if err != nil || len(history) != versions {
				t.Fatalf("deletion changed game history: %+v %v", history, err)
			}
		})
	}
}
