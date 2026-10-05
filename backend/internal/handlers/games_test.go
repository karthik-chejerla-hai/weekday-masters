package handlers

import (
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
	"net/http"
	"testing"
	"time"
)

func TestGameRoutesRequireApprovedMembership(t *testing.T) {
	h := newHarness(t)
	NewGameHandler(services.NewGameService()).RegisterRoutes(h.router.Group("/api"))
	routes := []struct{ method, path string }{
		{"GET", "/api/sessions/" + uuid.NewString() + "/games"}, {"POST", "/api/sessions/" + uuid.NewString() + "/games"},
		{"PUT", "/api/games/" + uuid.NewString()}, {"DELETE", "/api/games/" + uuid.NewString()},
		{"GET", "/api/games/" + uuid.NewString() + "/revisions"}, {"GET", "/api/games/players"}, {"GET", "/api/games/head-to-head"},
	}
	for _, actor := range []*models.User{nil, makePending(t)} {
		for _, r := range routes {
			res := h.as(actor).request(r.method, r.path, nil)
			if res.Code != 401 && res.Code != 403 {
				t.Fatalf("%s %s allowed %d", r.method, r.path, res.Code)
			}
		}
	}
}
func TestGameHTTPFlow(t *testing.T) {
	h := newHarness(t)
	NewGameHandler(services.NewGameService()).RegisterRoutes(h.router.Group("/api"))
	p := []*models.User{makePlayer(t), makePlayer(t), makePlayer(t), makePlayer(t)}
	se := makeSession(t, p[0].ID, 1)
	database.DB.Model(se).Updates(map[string]any{"session_date": time.Now().AddDate(0, 0, -1), "starts_at": time.Now().Add(-time.Hour)})
	in := map[string]any{"team_a": []uuid.UUID{p[0].ID, p[1].ID}, "team_b": []uuid.UUID{p[2].ID, p[3].ID}, "score_a": 21, "score_b": 17, "request_id": uuid.NewString()}
	var g services.GameView
	h.as(p[0]).post("/api/sessions/"+se.ID.String()+"/games", in).expect(http.StatusCreated).decode(&g)
	h.as(p[1]).put("/api/games/"+g.ID.String(), map[string]any{"team_a": in["team_a"], "team_b": in["team_b"], "score_a": 22, "score_b": 17, "version": 1}).expect(403)
	h.as(p[0]).get("/api/sessions/" + se.ID.String() + "/games").expect(200)
	h.get("/api/games/head-to-head?team_a=" + p[0].ID.String() + "&team_b=" + p[2].ID.String()).expect(200)
	h.get("/api/games/head-to-head?team_a=invalid&team_b=" + p[2].ID.String()).expect(400)
	h.get("/api/sessions/" + se.ID.String() + "/games?offset=-1").expect(400)
	h.get("/api/games/players").expect(200)
	h.request("DELETE", "/api/games/"+g.ID.String(), map[string]any{"version": 1}).expect(200)
	h.get("/api/games/" + g.ID.String() + "/revisions").expect(200)
	in["score_a"] = 21.5
	h.post("/api/sessions/"+se.ID.String()+"/games", in).expect(400)
}
