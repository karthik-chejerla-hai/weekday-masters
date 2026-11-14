package services

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxAnnouncementTitleLength = 200
const MaxAnnouncementBodyLength = 5000

var ErrAnnouncementRequestConflict = errors.New("announcement request ID was already used with different content")
var ErrAnnouncementNotAllowed = errors.New("only approved administrators can publish announcements")
var ErrAnnouncementInvalid = errors.New("a request ID, title up to 200 characters and message up to 5000 characters are required")

type AnnouncementPublication struct {
	models.Announcement
	DeliveryStatus string `json:"delivery_status"`
}

func announcementPublication(a models.Announcement) *AnnouncementPublication {
	status := "suppressed"
	if a.QueuedCount > 0 {
		status = "queued"
		if a.SuppressedCount > 0 {
			status = "partially_queued"
		}
	}
	return &AnnouncementPublication{a, status}
}

type AnnouncementService struct{ notifications *NotificationService }

func NewAnnouncementService(notifications *NotificationService) *AnnouncementService {
	if notifications == nil {
		notifications = &NotificationService{}
	}
	return &AnnouncementService{notifications: notifications}
}

// Publish saves publication, immutable audience history and outbox atomically.
// Provider availability cannot affect successful publication.
func (s *AnnouncementService) Publish(actorID, requestID uuid.UUID, title, body string) (*AnnouncementPublication, error) {
	title, body = strings.TrimSpace(title), strings.TrimSpace(body)
	if requestID == uuid.Nil || title == "" || body == "" || utf8.RuneCountInString(title) > MaxAnnouncementTitleLength || utf8.RuneCountInString(body) > MaxAnnouncementBodyLength {
		return nil, ErrAnnouncementInvalid
	}
	var result *AnnouncementPublication
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var actor models.User
		if err := tx.First(&actor, "id = ?", actorID).Error; err != nil {
			return err
		}
		if actor.MembershipStatus != models.MembershipApproved || actor.Role != models.RoleAdmin {
			return ErrAnnouncementNotAllowed
		}
		var err error
		result, err = s.publishWithin(tx, actorID, "announcement:"+actorID.String()+":"+requestID.String(), &requestID, title, body, nil)
		return err
	})
	return result, err
}
func (s *AnnouncementService) publishWithin(tx *gorm.DB, actorID uuid.UUID, key string, requestID *uuid.UUID, title, body string, sessionID *uuid.UUID) (*AnnouncementPublication, error) {
	if err := lockNotificationKey(tx, "publication:"+key); err != nil {
		return nil, err
	}
	var a models.Announcement
	err := tx.Where("publication_key = ?", key).First(&a).Error
	if err == nil {
		if a.Title != title || a.Body != body || a.CreatedBy != actorID {
			return nil, ErrAnnouncementRequestConflict
		}
		return announcementPublication(a), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	now := time.Now().UTC()
	a = models.Announcement{PublicationKey: &key, RequestID: requestID, Title: title, Body: body, CreatedBy: actorID, SentAt: now}
	if err := tx.Create(&a).Error; err != nil {
		return nil, err
	}
	var members []models.User
	if err := tx.Select("id").Where("membership_status = ?", models.MembershipApproved).Order("id").Find(&members).Error; err != nil {
		return nil, err
	}
	// Each immutable recipient notification is the persisted publication audience.
	// Approval/enrollment changes and request retries never extend this snapshot.
	data := map[string]string{"type": string(models.NotificationAdminAnnouncement), "announcement_id": a.ID.String()}
	if sessionID != nil {
		data["session_id"] = sessionID.String()
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		n := models.Notification{UserID: member.ID, NotificationType: models.NotificationAdminAnnouncement, Title: a.Title, Body: a.Body, Data: string(encoded)}
		outbox, err := s.notifications.QueueNotificationWithin(tx, &n, "announcement-recipient:"+a.ID.String()+":"+member.ID.String(), now, now.Add(24*time.Hour))
		if err != nil {
			return nil, err
		}
		a.AudienceCount++
		if outbox.State == models.DeliveryPending {
			a.QueuedCount++
		} else {
			a.SuppressedCount++
		}
	}
	if err := tx.Model(&a).Select("audience_count", "queued_count", "suppressed_count").Updates(&a).Error; err != nil {
		return nil, err
	}
	return announcementPublication(a), nil
}
