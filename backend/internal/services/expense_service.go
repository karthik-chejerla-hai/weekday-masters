package services

import (
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
	"sort"
)

type ExpenseInput struct {
	TotalHours          int         `json:"total_hours"`
	ShuttlesUsed        *int        `json:"shuttles_used"`
	ParticipantIDs      []uuid.UUID `json:"participant_ids,omitempty"`
	ExtraParticipantIDs []uuid.UUID `json:"extra_participant_ids,omitempty"`
	ExpectedPreview     string      `json:"expected_preview,omitempty"`
}
type ExpensePreview struct {
	Session               SessionSummary     `json:"session"`
	Input                 ExpenseInput       `json:"input"`
	Settlement            *SettlementPreview `json:"settlement"`
	Fingerprint           string             `json:"fingerprint"`
	CourtCreditAfterCents int64              `json:"court_credit_after_cents"`
	NextCourtCostCents    int64              `json:"next_court_cost_cents"`
	CourtTopupNeeded      bool               `json:"court_topup_needed"`
}
type ExpenseService struct {
	settlement *SettlementService
	ledger     *LedgerService
}

func NewExpenseService(settlement *SettlementService, ledger *LedgerService) *ExpenseService {
	return &ExpenseService{settlement: settlement, ledger: ledger}
}

func requireExpenseAdmin(actor *models.User) error {
	if actor == nil {
		return newLedgerError("unauthorized", 401, "Sign in to continue.")
	}
	if !actor.IsApproved() || actor.Role != models.RoleAdmin {
		return newLedgerError("forbidden", 403, "Only an approved admin can record expenses.")
	}
	return nil
}

func (s *ExpenseService) input(id uuid.UUID, in ExpenseInput, actor *models.User) (SettleInput, error) {
	if err := requireExpenseAdmin(actor); err != nil {
		return SettleInput{}, err
	}
	if in.TotalHours != 2 && in.TotalHours != 3 {
		return SettleInput{}, ErrNotSettleable("Choose two or three hours.")
	}
	if in.ShuttlesUsed == nil || *in.ShuttlesUsed < 0 || *in.ShuttlesUsed > 200 {
		return SettleInput{}, ErrNotSettleable("Enter the actual shuttle count from 0 to 200.")
	}
	base, extra := 2.0, float64(in.TotalHours-2)
	result := SettleInput{SessionID: id, SettledBy: actor.ID, BaseHours: &base, ExtraHours: &extra, ActualShuttles: in.ShuttlesUsed, ExpectedPreview: in.ExpectedPreview}
	ids := append([]uuid.UUID(nil), in.ParticipantIDs...)
	if in.ParticipantIDs == nil {
		var rsvps []models.RSVP
		if err := database.DB.Where("session_id = ? AND status = ?", id, models.RSVPStatusIn).Find(&rsvps).Error; err != nil {
			return result, err
		}
		for _, r := range rsvps {
			ids = append(ids, r.UserID)
		}
	}
	if len(ids) == 0 || len(ids) > 100 {
		return result, ErrNotSettleable("Choose between 1 and 100 members to charge.")
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	extraIDs := in.ExtraParticipantIDs
	if in.TotalHours == 2 && extraIDs != nil {
		return result, ErrNotSettleable("Choose three hours before selecting extra-hour players.")
	}
	if in.TotalHours == 3 && extraIDs == nil {
		extraIDs = ids
	}
	if in.TotalHours == 3 && len(extraIDs) == 0 {
		return result, ErrNotSettleable("Select at least one extra-hour player, or choose two hours.")
	}
	participants := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		participants[id] = true
	}
	extraPlayers := make(map[uuid.UUID]bool, len(extraIDs))
	for _, id := range extraIDs {
		if !participants[id] || extraPlayers[id] {
			return result, ErrNotSettleable("Each extra-hour player must be selected once in the main group.")
		}
		extraPlayers[id] = true
	}
	for i, userID := range ids {
		if userID == uuid.Nil || (i > 0 && ids[i-1] == userID) {
			return result, ErrNotSettleable("Each member can appear only once.")
		}
		result.Lines = append(result.Lines, LineInput{UserID: userID, InBase: true, InExtra: extraPlayers[userID]})
	}
	return result, validateExpenseMembers(database.DB, result.Lines)
}
func validateExpenseMembers(db *gorm.DB, lines []LineInput) error {
	ids := make([]uuid.UUID, 0, len(lines))
	for _, line := range lines {
		ids = append(ids, line.UserID)
	}
	var count int64
	if err := db.Model(&models.User{}).Where("id IN ? AND membership_status = ?", ids, models.MembershipApproved).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(ids)) {
		return ErrNotSettleable("Each participant must be an approved club member.")
	}
	return nil
}
func (s *ExpenseService) Preview(id uuid.UUID, in ExpenseInput, actor *models.User) (*ExpensePreview, error) {
	mapped, err := s.input(id, in, actor)
	if err != nil {
		return nil, err
	}
	var session models.Session
	if err = database.DB.First(&session, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if session.EndsAt == nil || !session.EndsAt.Before(utils.NowInSydney()) || session.Status == models.SessionStatusCancelled {
		return nil, ErrNotSettleable("Choose a finished session that was not cancelled.")
	}
	live, err := s.settlement.LiveSettlementForSession(id)
	if err != nil {
		return nil, err
	}
	if live != nil {
		return nil, ErrSessionAlreadySettled()
	}
	preview, err := s.settlement.Preview(mapped)
	if err != nil {
		return nil, err
	}
	position, err := s.ledger.Position()
	if err != nil {
		return nil, err
	}
	if position.AssetsPending {
		return nil, ErrNotSettleable("Confirm club assets before recording expenses.")
	}
	var club models.Club
	if err = database.DB.First(&club).Error; err != nil {
		return nil, err
	}
	canonical := ExpenseInput{TotalHours: in.TotalHours, ShuttlesUsed: in.ShuttlesUsed, ParticipantIDs: make([]uuid.UUID, 0, len(mapped.Lines))}
	for _, line := range mapped.Lines {
		canonical.ParticipantIDs = append(canonical.ParticipantIDs, line.UserID)
		if line.InExtra {
			canonical.ExtraParticipantIDs = append(canonical.ExtraParticipantIDs, line.UserID)
		}
	}
	after := position.Assets.CourtCreditCents - preview.Totals.CourtCents
	next := 2 * club.BaseRateCents
	return &ExpensePreview{Session: SessionSummary{ID: session.ID, Title: session.Title, StartsAt: session.StartsAt, EndsAt: session.EndsAt}, Input: canonical, Settlement: preview, Fingerprint: preview.Fingerprint, CourtCreditAfterCents: after, NextCourtCostCents: next, CourtTopupNeeded: after < next}, nil
}
func (s *ExpenseService) Confirm(id uuid.UUID, in ExpenseInput, actor *models.User) (*models.Settlement, error) {
	mapped, err := s.input(id, in, actor)
	if err != nil {
		return nil, err
	}
	if len(in.ExpectedPreview) != 64 {
		return nil, newLedgerError("preview_required", 409, "Review an expense preview before confirming.")
	}
	record, _, err := s.settlement.Settle(mapped)
	return record, err
}

// ListUnsettledSessions uses a separate query, so settled rows and history paging
// cannot hide older outstanding sessions. RSVP count is meaningful before charges exist.
func (s *ExpenseService) ListUnsettledSessions() ([]PastSessionView, error) {
	items := []PastSessionView{}
	err := database.DB.Raw(`SELECT s.id AS session_id,s.title,s.starts_at,s.ends_at,false AS settled,
 0 AS total_cents,(SELECT COUNT(*) FROM rsvps r WHERE r.session_id=s.id AND r.status='in') AS player_count
 FROM sessions s WHERE s.ends_at < ? AND s.status != ?
 AND NOT EXISTS (SELECT 1 FROM settlements st WHERE st.session_id=s.id AND st.reversed_at IS NULL)
 ORDER BY s.ends_at ASC,s.id ASC`, utils.NowInSydney(), models.SessionStatusCancelled).Scan(&items).Error
	return items, err
}
