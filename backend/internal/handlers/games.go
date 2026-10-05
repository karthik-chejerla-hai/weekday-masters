package handlers

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/services"
	"gorm.io/gorm"
	"strconv"
	"strings"
)

type GameHandler struct{ service *services.GameService }

func NewGameHandler(s *services.GameService) *GameHandler { return &GameHandler{service: s} }
func (h *GameHandler) RegisterRoutes(protected *gin.RouterGroup) {
	approved := protected.Group("", middleware.RequireApproved())
	approved.GET("/sessions/:id/games", h.List)
	approved.POST("/sessions/:id/games", h.Create)
	approved.GET("/games/players", h.Players)
	approved.GET("/games/head-to-head", h.HeadToHead)
	approved.PUT("/games/:gameId", h.Update)
	approved.DELETE("/games/:gameId", h.Void)
	approved.GET("/games/:gameId/revisions", h.Revisions)
}
func gameID(c *gin.Context, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		c.JSON(400, gin.H{"code": "bad_request", "message": "Invalid game or session ID."})
		return uuid.Nil, false
	}
	return id, true
}
func gameOffset(c *gin.Context) (int, bool) {
	n, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || n < 0 {
		c.JSON(400, gin.H{"code": "bad_request", "message": "Invalid page offset."})
		return 0, false
	}
	return n, true
}
func respondGameError(c *gin.Context, err error) {
	if _, ok := services.AsLedgerError(err); ok {
		respondLedgerError(c, err)
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(404, gin.H{"code": "not_found", "message": "This game or session is not available."})
		return
	}
	c.JSON(500, gin.H{"code": "internal", "message": "Could not load or save the game. Try again."})
}
func (h *GameHandler) List(c *gin.Context) {
	id, ok := gameID(c, "id")
	if !ok {
		return
	}
	offset, ok := gameOffset(c)
	if !ok {
		return
	}
	result, err := h.service.List(c.Request.Context(), id, offset)
	if err != nil {
		respondGameError(c, err)
		return
	}
	c.JSON(200, result)
}
func (h *GameHandler) Create(c *gin.Context) { h.save(c, true) }
func (h *GameHandler) Update(c *gin.Context) { h.save(c, false) }
func (h *GameHandler) save(c *gin.Context, create bool) {
	param := "gameId"
	if create {
		param = "id"
	}
	id, ok := gameID(c, param)
	if !ok {
		return
	}
	var in services.GameInput
	if err := decodeBoundedJSON(c, &in, 8<<10); err != nil {
		c.JSON(400, gin.H{"code": "bad_request", "message": "Check the players and whole-number scores."})
		return
	}
	actor, _ := middleware.GetUserFromContext(c)
	var result *services.GameView
	var err error
	if create {
		result, err = h.service.Create(c.Request.Context(), id, in, actor)
	} else {
		result, err = h.service.Update(c.Request.Context(), id, in, actor)
	}
	if err != nil {
		respondGameError(c, err)
		return
	}
	status := 200
	if create {
		status = 201
	}
	c.JSON(status, result)
}
func (h *GameHandler) Void(c *gin.Context) {
	id, ok := gameID(c, "gameId")
	if !ok {
		return
	}
	var in struct {
		Version int `json:"version"`
	}
	if err := decodeBoundedJSON(c, &in, 1024); err != nil {
		c.JSON(400, gin.H{"code": "bad_request", "message": "Include the current result version."})
		return
	}
	actor, _ := middleware.GetUserFromContext(c)
	result, err := h.service.Void(c.Request.Context(), id, in.Version, actor)
	if err != nil {
		respondGameError(c, err)
		return
	}
	c.JSON(200, result)
}
func (h *GameHandler) Revisions(c *gin.Context) {
	id, ok := gameID(c, "gameId")
	if !ok {
		return
	}
	result, err := h.service.Revisions(c.Request.Context(), id)
	if err != nil {
		respondGameError(c, err)
		return
	}
	c.JSON(200, result)
}
func (h *GameHandler) Players(c *gin.Context) {
	result, err := h.service.Players(c.Request.Context())
	if err != nil {
		respondGameError(c, err)
		return
	}
	c.JSON(200, result)
}
func (h *GameHandler) HeadToHead(c *gin.Context) {
	offset, ok := gameOffset(c)
	if !ok {
		return
	}
	teams := [][]uuid.UUID{}
	for _, name := range []string{"team_a", "team_b"} {
		parts := strings.Split(c.Query(name), ",")
		ids := []uuid.UUID{}
		for _, part := range parts {
			id, err := uuid.Parse(part)
			if err != nil {
				c.JSON(400, gin.H{"code": "bad_request", "message": "Choose valid opponents."})
				return
			}
			ids = append(ids, id)
		}
		teams = append(teams, ids)
	}
	result, err := h.service.HeadToHead(c.Request.Context(), teams[0], teams[1], offset)
	if err != nil {
		respondGameError(c, err)
		return
	}
	c.JSON(200, result)
}
