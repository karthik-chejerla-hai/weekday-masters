package handlers

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/services"
	"net/http"
)

func (h *NotificationHandler) RegisterDeviceRoutes(protected *gin.RouterGroup) {
	approved := protected.Group("", middleware.RequireApproved())
	approved.GET("/users/me/push-devices", h.GetPushDevices)
	approved.POST("/users/me/push-devices/enroll", h.EnrollPushDevice)
	approved.POST("/users/me/push-devices/adopt", h.AdoptPushDevice)
	approved.PUT("/users/me/push-devices/:id/registration", h.RefreshPushDevice)
	approved.DELETE("/users/me/push-devices/:id", h.RevokePushDevice)
	// Authenticated revoke-only exception: pending/removed new accounts must be
	// able to retire the former browser registration without the former JWT.
	protected.POST("/push-devices/revoke-proof", h.RevokePushDeviceProof)
}

type pushDeviceRequest struct {
	InstallationID uuid.UUID `json:"installation_id" binding:"required"`
	Token          string    `json:"token" binding:"required,max=4096"`
	Secret         string    `json:"revocation_secret" binding:"required,len=64"`
	Generation     int64     `json:"generation" binding:"min=0"`
	DeviceName     string    `json:"device_name" binding:"max=200"`
}

func deviceError(c *gin.Context, err error) {
	if errors.Is(err, services.ErrDeviceNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": "device_not_found", "message": "Device registration unavailable."})
		return
	}
	if errors.Is(err, services.ErrDeviceConflict) {
		c.JSON(http.StatusConflict, gin.H{"code": "device_changed", "message": err.Error()})
		return
	}
	var sqlError interface{ SQLState() string }
	if errors.As(err, &sqlError) && sqlError.SQLState() == "23505" {
		c.JSON(http.StatusConflict, gin.H{"code": "device_registration_failed", "message": "This device cannot be registered with those details."})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"code": "device_unavailable", "message": "Device setup is temporarily unavailable. Try again."})
}
func (h *NotificationHandler) GetPushDevices(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	devices, err := h.notificationService.GetPushDevices(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load devices"})
		return
	}
	c.JSON(http.StatusOK, devices)
}
func (h *NotificationHandler) EnrollPushDevice(c *gin.Context) { h.savePushDevice(c, false) }
func (h *NotificationHandler) AdoptPushDevice(c *gin.Context)  { h.savePushDevice(c, true) }
func (h *NotificationHandler) savePushDevice(c *gin.Context, adopt bool) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	var req pushDeviceRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device registration"})
		return
	}
	var device *services.PushEnrollment
	if adopt {
		device, err = h.notificationService.AdoptPushDevice(user.ID, req.InstallationID, req.Token, req.Secret)
	} else {
		device, err = h.notificationService.EnrollPushDevice(user.ID, req.InstallationID, req.Token, req.Secret, req.DeviceName, req.Generation)
	}
	if err != nil {
		deviceError(c, err)
		return
	}
	c.JSON(http.StatusOK, device)
}
func (h *NotificationHandler) RefreshPushDevice(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	var req struct {
		InstallationID uuid.UUID `json:"installation_id" binding:"required"`
		Token          string    `json:"token" binding:"required,max=4096"`
		Generation     int64     `json:"generation" binding:"required,min=1"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device registration"})
		return
	}
	device, err := h.notificationService.RefreshPushDevice(user.ID, id, req.InstallationID, req.Token, req.Generation)
	if err != nil {
		deviceError(c, err)
		return
	}
	c.JSON(http.StatusOK, device)
}
func (h *NotificationHandler) RevokePushDevice(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	var req struct {
		Generation int64 `json:"generation" binding:"required,min=1"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if err := h.notificationService.RevokePushDevice(user.ID, id, req.Generation); err != nil {
		deviceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *NotificationHandler) RevokePushDeviceProof(c *gin.Context) {
	if _, err := middleware.GetUserFromContext(c); err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	var req struct {
		InstallationID uuid.UUID `json:"installation_id" binding:"required"`
		Secret         string    `json:"revocation_secret" binding:"required,len=64"`
		Generation     int64     `json:"generation" binding:"required,min=1"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if err := h.notificationService.RevokePushDeviceProof(req.InstallationID, req.Secret, req.Generation); err != nil {
		deviceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *NotificationHandler) ObsoletePushMutation(c *gin.Context) {
	if _, err := middleware.GetUserFromContext(c); err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	c.JSON(http.StatusConflict, gin.H{"code": "update_required", "message": "Refresh Rally to manage notifications on this device."})
}
