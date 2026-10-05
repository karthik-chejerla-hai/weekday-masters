package handlers

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/assistant"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/services"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type AssistantHandler struct {
	service *services.AssistantService
	mu      sync.Mutex
	active  map[uuid.UUID]bool
}

func NewAssistantHandler(service *services.AssistantService) *AssistantHandler {
	return &AssistantHandler{service: service, active: map[uuid.UUID]bool{}}
}
func (h *AssistantHandler) RegisterRoutes(protected *gin.RouterGroup) {
	approved := protected.Group("/assistant", middleware.RequireApproved())
	approved.GET("/status", func(c *gin.Context) { c.JSON(200, gin.H{"enabled": h.service.Enabled()}) })
	approved.POST("/messages", h.Messages)
	approved.POST("/transcribe", h.Transcribe)
}

// One active provider request per member prevents duplicate clicks from consuming quota.
func (h *AssistantHandler) acquire(c *gin.Context) (func(), bool) {
	actor, err := middleware.GetUserFromContext(c)
	if err != nil {
		return nil, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.active[actor.ID] {
		c.JSON(429, gin.H{"code": "assistant_busy", "message": "Your previous assistant request is still running."})
		return nil, false
	}
	h.active[actor.ID] = true
	return func() { h.mu.Lock(); delete(h.active, actor.ID); h.mu.Unlock() }, true
}
func (h *AssistantHandler) Messages(c *gin.Context) {
	var in services.AssistantInput
	if err := decodeBoundedJSON(c, &in, 24<<10); err != nil {
		c.JSON(400, gin.H{"code": "bad_request", "message": "Send a short text request."})
		return
	}
	release, ok := h.acquire(c)
	if !ok {
		return
	}
	defer release()
	actor, _ := middleware.GetUserFromContext(c)
	reply, err := h.service.Reply(c.Request.Context(), in, actor)
	if err != nil {
		respondAssistantError(c, err)
		return
	}
	c.JSON(200, reply)
}
func (h *AssistantHandler) Transcribe(c *gin.Context) {
	if !h.service.Enabled() {
		respondAssistantError(c, assistant.Unavailable())
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, assistant.MaxAudioBytes+(64<<10))
	if err := c.Request.ParseMultipartForm(assistant.MaxAudioBytes); err != nil {
		c.JSON(400, gin.H{"code": "invalid_audio", "message": "Use a recording smaller than 10 MB."})
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"code": "invalid_audio", "message": "Include an audio recording."})
		return
	}
	defer file.Close()
	mimeType := header.Header.Get("Content-Type")
	if _, ok := assistant.AudioExtension(mimeType); !ok {
		c.JSON(400, gin.H{"code": "invalid_audio", "message": "This recording format is not supported. Try another browser or type your request."})
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, assistant.MaxAudioBytes+1))
	if err != nil || len(data) == 0 || len(data) > assistant.MaxAudioBytes {
		c.JSON(400, gin.H{"code": "invalid_audio", "message": "Use a non-empty recording smaller than 10 MB."})
		return
	}
	release, ok := h.acquire(c)
	if !ok {
		return
	}
	defer release()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 40*time.Second)
	defer cancel()
	transcript, err := h.service.Transcribe(ctx, assistant.Audio{Data: data, MIME: mimeType})
	if err != nil {
		respondAssistantError(c, err)
		return
	}
	c.JSON(200, gin.H{"text": transcript})
}
func respondAssistantError(c *gin.Context, err error) {
	var failure *assistant.Error
	if errors.As(err, &failure) {
		if failure.RetryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(failure.RetryAfter))
		}
		c.JSON(failure.Status, gin.H{"code": failure.Code, "message": failure.Message})
		return
	}
	respondExpenseError(c, err)
}
