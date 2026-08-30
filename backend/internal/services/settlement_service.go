package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services/money"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SettlementService turns a played session into charges.
//
// It composes on top of LedgerService rather than writing entries itself: it
// works out who owes what, and hands the resulting movements over to the one
// thing allowed to touch the ledger.
type SettlementService struct {
	ledger   *LedgerService
	notifier BalanceNotifier
}

func NewSettlementService(ledger *LedgerService) *SettlementService {
	return &SettlementService{ledger: ledger}
}

// LineInput is one participant as the admin has them on the settlement form.
type LineInput struct {
	UserID    uuid.UUID `json:"user_id"`
	GuestName string    `json:"guest_name,omitempty"`
	InBase    bool      `json:"in_base"`
	InExtra   bool      `json:"in_extra"`
	Comped    bool      `json:"comped"`
}

// SettleInput describes a settlement. Rates left at zero fall back to club
// settings, so the common case is submitting the participant list and nothing
// else.
type SettleInput struct {
	SessionID       uuid.UUID
	BaseHours       *float64
	BaseRateCents   *int64
	ExtraHours      *float64
	ExtraRateCents  *int64
	ShuttlesPerHour *float64
	Lines           []LineInput
	SettledBy       uuid.UUID
}

// BandView is what one band of the night cost and how it was shared.
type BandView struct {
	Hours        float64 `json:"hours"`
	CourtCents   int64   `json:"court_cents"`
	ShuttleUnits int     `json:"shuttle_units"`
	ShuttleCents int64   `json:"shuttle_cents"`
	TotalCents   int64   `json:"total_cents"`
	Heads        int     `json:"heads"`
}

// ChargeLineView is a costed participant line.
type ChargeLineView struct {
	UserID      uuid.UUID `json:"user_id"`
	Name        string    `json:"name,omitempty"`
	GuestName   string    `json:"guest_name,omitempty"`
	InBase      bool      `json:"in_base"`
	InExtra     bool      `json:"in_extra"`
	Comped      bool      `json:"comped"`
	AmountCents int64     `json:"amount_cents"`
}

// SettlementTotals is the whole night in one row.
type SettlementTotals struct {
	CourtCents   int64 `json:"court_cents"`
	ShuttleCents int64 `json:"shuttle_cents"`
	ShuttleUnits int   `json:"shuttle_units"`
	ChargedCents int64 `json:"charged_cents"`
	SurplusCents int64 `json:"surplus_cents"`
}

// StockView is the shuttle bag after this settlement.
type StockView struct {
	Units       int   `json:"units"`
	AmountCents int64 `json:"amount_cents"`
}

// SettlementPreview is what the form shows, and exactly what settling will post.
type SettlementPreview struct {
	Bands      map[string]*BandView `json:"bands"`
	Totals     SettlementTotals     `json:"totals"`
	Lines      []ChargeLineView     `json:"lines"`
	StockAfter StockView            `json:"stock_after"`
}

// rates carries the values actually used, after club defaults are applied.
type rates struct {
	baseHours       float64
	baseRateCents   int64
	extraHours      float64
	extraRateCents  int64
	shuttlesPerHour float64
}

func (r rates) baseCourtCents() int64  { return int64(r.baseHours * float64(r.baseRateCents)) }
func (r rates) extraCourtCents() int64 { return int64(r.extraHours * float64(r.extraRateCents)) }
func (r rates) baseShuttles() int      { return int(r.baseHours * r.shuttlesPerHour) }
func (r rates) extraShuttles() int     { return int(r.extraHours * r.shuttlesPerHour) }
func (r rates) hasExtra() bool         { return r.extraHours > 0 }

// resolveRates fills anything the caller left unset from club settings.
func (s *SettlementService) resolveRates(in SettleInput) (rates, error) {
	var club models.Club
	if err := database.DB.First(&club).Error; err != nil {
		return rates{}, fmt.Errorf("settlement: club settings unavailable: %w", err)
	}

	r := rates{
		baseHours:       club.BaseHours,
		baseRateCents:   club.BaseRateCents,
		extraRateCents:  club.ExtraRateCents,
		shuttlesPerHour: club.ShuttlesPerHour,
	}
	if in.BaseHours != nil {
		r.baseHours = *in.BaseHours
	}
	if in.BaseRateCents != nil {
		r.baseRateCents = *in.BaseRateCents
	}
	if in.ExtraHours != nil {
		r.extraHours = *in.ExtraHours
	}
	if in.ExtraRateCents != nil {
		r.extraRateCents = *in.ExtraRateCents
	}
	if in.ShuttlesPerHour != nil {
		r.shuttlesPerHour = *in.ShuttlesPerHour
	}

	if r.baseHours <= 0 {
		return rates{}, ErrNotSettleable("A session must have at least some standard hours.")
	}
	if r.extraHours < 0 || r.baseRateCents < 0 || r.extraRateCents < 0 || r.shuttlesPerHour < 0 {
		return rates{}, ErrNotSettleable("Rates and hours cannot be negative.")
	}
	return r, nil
}

// defaultLines builds the form's starting state: everyone who said they were
// coming, in every band of the session.
//
// No-shows are included deliberately. The court was booked for them, so the
// default is that they pay; the admin removes anyone they decide not to charge.
func (s *SettlementService) defaultLines(sessionID uuid.UUID, hasExtra bool) ([]LineInput, error) {
	var rsvps []models.RSVP
	if err := database.DB.
		Where("session_id = ? AND status IN ?", sessionID,
			[]models.RSVPStatus{models.RSVPStatusIn, models.RSVPStatusWaitlisted}).
		Order("rsvp_timestamp ASC").
		Find(&rsvps).Error; err != nil {
		return nil, err
	}

	lines := make([]LineInput, 0, len(rsvps))
	for _, rsvp := range rsvps {
		// A waitlisted player who never got promoted did not play.
		if rsvp.Status != models.RSVPStatusIn {
			continue
		}
		lines = append(lines, LineInput{UserID: rsvp.UserID, InBase: true, InExtra: hasExtra})
	}
	return lines, nil
}

// Preview costs a settlement without writing anything.
//
// The form re-previews on every change, so what the admin is looking at is
// always what pressing settle will post.
func (s *SettlementService) Preview(in SettleInput) (*SettlementPreview, error) {
	r, err := s.resolveRates(in)
	if err != nil {
		return nil, err
	}

	lines := in.Lines
	if lines == nil {
		lines, err = s.defaultLines(in.SessionID, r.hasExtra())
		if err != nil {
			return nil, err
		}
	}

	stock, err := s.ledger.StockPosition(nil)
	if err != nil {
		return nil, err
	}

	preview, _, err := s.cost(in.SessionID, r, lines, stock)
	if err != nil {
		return nil, err
	}
	return preview, nil
}

// bandCharge is one participant's share of one band, before comping.
type bandCharge struct {
	lineIndex int
	amount    int64
}

// cost works out the whole settlement: band totals, shuttle consumption, and
// each line's share. It writes nothing.
//
// Returns the preview plus the per-line amounts, so Settle can post exactly what
// Preview displayed.
func (s *SettlementService) cost(
	sessionID uuid.UUID,
	r rates,
	lines []LineInput,
	stock money.Stock,
) (*SettlementPreview, []int64, error) {
	if err := validateLines(lines, r.hasExtra()); err != nil {
		return nil, nil, err
	}

	// Shuttles come out of the bag before anything is charged, because the cost
	// of a shuttle depends on what is in the bag at that moment.
	baseUnits := r.baseShuttles()
	extraUnits := r.extraShuttles()

	baseShuttleCents, afterBase, err := stock.Consume(baseUnits)
	if err != nil {
		return nil, nil, wrapStockShortfall(err, baseUnits+extraUnits, stock.Units)
	}
	extraShuttleCents, afterExtra, err := afterBase.Consume(extraUnits)
	if err != nil {
		return nil, nil, wrapStockShortfall(err, baseUnits+extraUnits, stock.Units)
	}

	baseTotal := r.baseCourtCents() + baseShuttleCents
	extraTotal := r.extraCourtCents() + extraShuttleCents

	amounts := make([]int64, len(lines))
	var comped int64

	// Each band is split among only its own participants. A player who went home
	// after the standard hours contributes nothing to the extension — flat
	// pro-rata across the whole night would make them subsidise it.
	baseCharges, err := splitBand(sessionID, baseTotal, lines, func(l LineInput) bool { return l.InBase })
	if err != nil {
		return nil, nil, err
	}
	extraCharges, err := splitBand(sessionID, extraTotal, lines, func(l LineInput) bool { return l.InExtra })
	if err != nil {
		return nil, nil, err
	}

	for _, charge := range append(baseCharges, extraCharges...) {
		amounts[charge.lineIndex] += charge.amount
	}

	// A comped player still counted as a head above, so nobody else's share
	// moved. Their waived amount is absorbed by the club rather than quietly
	// redistributed to the others.
	for i, line := range lines {
		if line.Comped {
			comped += amounts[i]
			amounts[i] = 0
		}
	}

	var charged int64
	for _, amount := range amounts {
		charged += amount
	}

	views, err := s.lineViews(lines, amounts)
	if err != nil {
		return nil, nil, err
	}

	bands := map[string]*BandView{
		"base": {
			Hours:        r.baseHours,
			CourtCents:   r.baseCourtCents(),
			ShuttleUnits: baseUnits,
			ShuttleCents: baseShuttleCents,
			TotalCents:   baseTotal,
			Heads:        countHeads(lines, func(l LineInput) bool { return l.InBase }),
		},
	}
	if r.hasExtra() {
		bands["extra"] = &BandView{
			Hours:        r.extraHours,
			CourtCents:   r.extraCourtCents(),
			ShuttleUnits: extraUnits,
			ShuttleCents: extraShuttleCents,
			TotalCents:   extraTotal,
			Heads:        countHeads(lines, func(l LineInput) bool { return l.InExtra }),
		}
	}

	preview := &SettlementPreview{
		Bands: bands,
		Totals: SettlementTotals{
			CourtCents:   r.baseCourtCents() + r.extraCourtCents(),
			ShuttleCents: baseShuttleCents + extraShuttleCents,
			ShuttleUnits: baseUnits + extraUnits,
			ChargedCents: charged,
			SurplusCents: -comped,
		},
		Lines:      views,
		StockAfter: StockView{Units: afterExtra.Units, AmountCents: afterExtra.ValueCents},
	}
	return preview, amounts, nil
}

// splitBand divides a band's cost equally among its participants.
func splitBand(seed uuid.UUID, total int64, lines []LineInput, in func(LineInput) bool) ([]bandCharge, error) {
	indexByParticipant := map[uuid.UUID]int{}
	participants := make([]uuid.UUID, 0, len(lines))

	for i, line := range lines {
		if !in(line) {
			continue
		}
		// A member and each of their guests are separate heads, so each line
		// needs its own identity in the split even when the account is shared.
		key := uuid.NewSHA1(seed, []byte(fmt.Sprintf("%s|%s|%d", line.UserID, line.GuestName, i)))
		indexByParticipant[key] = i
		participants = append(participants, key)
	}

	if len(participants) == 0 || total == 0 {
		return nil, nil
	}

	shares, err := money.SplitLargestRemainder(total, participants, seed)
	if err != nil {
		return nil, err
	}

	charges := make([]bandCharge, 0, len(shares))
	for _, share := range shares {
		charges = append(charges, bandCharge{
			lineIndex: indexByParticipant[share.ParticipantID],
			amount:    share.AmountCents,
		})
	}
	return charges, nil
}

func countHeads(lines []LineInput, in func(LineInput) bool) int {
	var n int
	for _, line := range lines {
		if in(line) {
			n++
		}
	}
	return n
}

func validateLines(lines []LineInput, hasExtra bool) error {
	for _, line := range lines {
		if line.UserID == uuid.Nil {
			return ErrNotSettleable("Every line must belong to a member.")
		}
		if !line.InBase && !line.InExtra {
			return ErrNotSettleable("A line that took part in nothing is not a line — remove it instead.")
		}
		if line.InExtra && !hasExtra {
			return ErrNotSettleable("Someone is marked for the extra hour, but this session did not run one.")
		}
	}
	return nil
}

// wrapStockShortfall reports the shortfall for the whole night rather than the
// band that happened to run out, so the admin buys enough the first time.
func wrapStockShortfall(err error, required, available int) error {
	var short *money.InsufficientStockError
	if errors.As(err, &short) {
		return ErrShuttleStockShort(required, available)
	}
	return err
}

// lineViews decorates costed lines with member names for display.
func (s *SettlementService) lineViews(lines []LineInput, amounts []int64) ([]ChargeLineView, error) {
	ids := make([]uuid.UUID, 0, len(lines))
	for _, line := range lines {
		ids = append(ids, line.UserID)
	}

	names := map[uuid.UUID]string{}
	if len(ids) > 0 {
		var users []models.User
		if err := database.DB.Where("id IN ?", ids).Find(&users).Error; err != nil {
			return nil, err
		}
		for _, u := range users {
			names[u.ID] = u.DisplayName()
		}
	}

	views := make([]ChargeLineView, 0, len(lines))
	for i, line := range lines {
		views = append(views, ChargeLineView{
			UserID:      line.UserID,
			Name:        names[line.UserID],
			GuestName:   line.GuestName,
			InBase:      line.InBase,
			InExtra:     line.InExtra,
			Comped:      line.Comped,
			AmountCents: amounts[i],
		})
	}
	return views, nil
}

// Settle posts the settlement.
//
// Everything happens inside one database transaction that first locks the
// session row, mirroring the RSVP capacity pattern: under that lock the
// check-then-act sequence for "is this already settled?" is safe, and two
// simultaneous requests resolve to exactly one settlement.
func (s *SettlementService) Settle(in SettleInput) (*models.Settlement, *SettlementPreview, error) {
	var settlement *models.Settlement
	var preview *SettlementPreview

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var session models.Session
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&session, "id = ?", in.SessionID).Error; err != nil {
			return err
		}
		if session.Status == models.SessionStatusCancelled {
			return ErrNotSettleable("This session was cancelled, so there is nothing to settle.")
		}

		var live int64
		if err := tx.Model(&models.Settlement{}).
			Where("session_id = ? AND reversed_at IS NULL", in.SessionID).
			Count(&live).Error; err != nil {
			return err
		}
		if live > 0 {
			return ErrSessionAlreadySettled()
		}

		r, err := s.resolveRates(in)
		if err != nil {
			return err
		}

		lines := in.Lines
		if lines == nil {
			lines, err = s.defaultLines(in.SessionID, r.hasExtra())
			if err != nil {
				return err
			}
		}
		if len(lines) == 0 {
			return ErrNotSettleable("Nobody is on this settlement.")
		}

		stock, err := s.ledger.StockPosition(tx)
		if err != nil {
			return err
		}

		costed, amounts, err := s.cost(in.SessionID, r, lines, stock)
		if err != nil {
			return err
		}
		preview = costed

		movements, err := s.movements(tx, r, costed, lines, amounts)
		if err != nil {
			return err
		}

		txn, err := s.ledger.PostWithin(tx, PostInput{
			Kind:        models.TxnSessionSettlement,
			SessionID:   &in.SessionID,
			Description: settlementDescription(session, r),
			OccurredAt:  settlementOccurredAt(session),
			CreatedBy:   in.SettledBy,
			Movements:   movements,
		})
		if err != nil {
			return err
		}

		record := models.Settlement{
			SessionID:        in.SessionID,
			TransactionID:    txn.ID,
			BaseHours:        r.baseHours,
			BaseRateCents:    r.baseRateCents,
			ExtraHours:       r.extraHours,
			ExtraRateCents:   r.extraRateCents,
			ShuttlesPerHour:  r.shuttlesPerHour,
			BaseShuttleCents: costed.Bands["base"].ShuttleCents,
			BaseShuttleUnits: costed.Bands["base"].ShuttleUnits,
			SettledAt:        utils.NowInSydney(),
			SettledBy:        in.SettledBy,
		}
		if extra, ok := costed.Bands["extra"]; ok {
			record.ExtraShuttleCents = extra.ShuttleCents
			record.ExtraShuttleUnits = extra.ShuttleUnits
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}

		for i, line := range lines {
			chargeLine := models.ChargeLine{
				SettlementID: record.ID,
				UserID:       line.UserID,
				GuestName:    line.GuestName,
				InBase:       line.InBase,
				InExtra:      line.InExtra,
				Comped:       line.Comped,
				AmountCents:  amounts[i],
			}
			if err := tx.Create(&chargeLine).Error; err != nil {
				return err
			}
		}

		settlement = &record
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// After the commit, deliberately. A reminder about a balance that was then
	// rolled back would be a lie, and notification failures must not undo a
	// settlement that already happened.
	var session models.Session
	if err := database.DB.Select("title").First(&session, "id = ?", in.SessionID).Error; err == nil {
		s.notifyLowBalances(context.Background(), preview, session.Title)
	}

	return settlement, preview, nil
}

// movements turns a costed settlement into ledger movements.
//
// Notably absent: the bank. Settling moves nothing in or out of the club's cash
// — that happened earlier, when members topped up and when the admin bought
// court credit and shuttles. This only draws down what was already paid for.
func (s *SettlementService) movements(
	tx *gorm.DB,
	r rates,
	preview *SettlementPreview,
	lines []LineInput,
	amounts []int64,
) ([]Movement, error) {
	courtCredit, err := s.ledger.ClubAccountID(tx, models.AccountKindCourtCredit)
	if err != nil {
		return nil, err
	}
	shuttleStock, err := s.ledger.ClubAccountID(tx, models.AccountKindShuttleStock)
	if err != nil {
		return nil, err
	}

	units := -preview.Totals.ShuttleUnits
	movements := []Movement{
		{AccountID: courtCredit, AmountCents: -preview.Totals.CourtCents},
		{AccountID: shuttleStock, AmountCents: -preview.Totals.ShuttleCents, Units: &units},
	}

	// One movement per account, not per line — a member hosting a guest is
	// charged once for both.
	byAccount := map[uuid.UUID]int64{}
	order := make([]uuid.UUID, 0, len(lines))
	for i, line := range lines {
		if amounts[i] == 0 {
			continue
		}
		accountID, err := s.ledger.PlayerAccountID(line.UserID)
		if err != nil {
			return nil, err
		}
		if _, seen := byAccount[accountID]; !seen {
			order = append(order, accountID)
		}
		byAccount[accountID] += amounts[i]
	}
	for _, accountID := range order {
		movements = append(movements, Movement{AccountID: accountID, AmountCents: -byAccount[accountID]})
	}

	if preview.Totals.SurplusCents != 0 {
		surplus, err := s.ledger.ClubAccountID(tx, models.AccountKindSurplus)
		if err != nil {
			return nil, err
		}
		movements = append(movements, Movement{AccountID: surplus, AmountCents: preview.Totals.SurplusCents})
	}

	return movements, nil
}

func settlementDescription(session models.Session, r rates) string {
	if r.hasExtra() {
		return fmt.Sprintf("%s — %gh + %gh extra", session.Title, r.baseHours, r.extraHours)
	}
	return fmt.Sprintf("%s — %gh", session.Title, r.baseHours)
}

// settlementOccurredAt dates the charge to the night played, not the night the
// admin got round to entering it.
func settlementOccurredAt(session models.Session) time.Time {
	if session.EndsAt != nil {
		return session.EndsAt.In(utils.SydneyLocation)
	}
	return utils.NowInSydney()
}

// ReverseSettlement undoes a settled session.
//
// It reverses the underlying ledger transaction — which unwinds the charges,
// the court credit and the shuttles, count included — and stamps the settlement,
// which frees the session to be settled again. Both the original and its
// reversal stay visible: the point of an append-only ledger is that a correction
// is a second fact, not the erasure of the first.
func (s *SettlementService) ReverseSettlement(settlementID uuid.UUID, reason string, reversedBy uuid.UUID) (*models.Transaction, error) {
	var settlement models.Settlement
	if err := database.DB.First(&settlement, "id = ?", settlementID).Error; err != nil {
		return nil, err
	}
	if !settlement.IsLive() {
		return nil, ErrTransactionAlreadyReversed()
	}

	if reason == "" {
		reason = "Settlement reversed"
	}

	txn, err := s.ledger.ReverseTransaction(settlement.TransactionID, reason, reversedBy)
	if err != nil {
		return nil, err
	}

	now := utils.NowInSydney()
	if err := database.DB.Model(&models.Settlement{}).
		Where("id = ?", settlementID).
		Updates(map[string]any{"reversed_at": now, "reversed_by": reversedBy}).Error; err != nil {
		return nil, err
	}

	return txn, nil
}

// LiveSettlementForSession returns the settlement currently standing for a
// session, if there is one.
func (s *SettlementService) LiveSettlementForSession(sessionID uuid.UUID) (*models.Settlement, error) {
	var settlement models.Settlement
	err := database.DB.Preload("Lines").
		Where("session_id = ? AND reversed_at IS NULL", sessionID).
		First(&settlement).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &settlement, nil
}

// PastSessionView is one row of the history list.
type PastSessionView struct {
	SessionID   uuid.UUID  `json:"session_id"`
	Title       string     `json:"title"`
	StartsAt    *time.Time `json:"starts_at,omitempty"`
	EndsAt      *time.Time `json:"ends_at,omitempty"`
	Settled     bool       `json:"settled"`
	TotalCents  int64      `json:"total_cents"`
	PlayerCount int        `json:"player_count"`
}

// ListPastSessions returns sessions that have finished, newest first.
//
// A finished session nobody has costed yet still appears, marked unsettled. It
// is exactly the thing the admin needs reminding about, so hiding it would be
// the wrong kindness.
func (s *SettlementService) ListPastSessions(limit, offset int) ([]PastSessionView, int64, error) {
	now := utils.NowInSydney()

	var total int64
	if err := database.DB.Model(&models.Session{}).
		Where("ends_at IS NOT NULL AND ends_at < ?", now).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var views []PastSessionView
	err := database.DB.Raw(`
		SELECT s.id AS session_id,
		       s.title,
		       s.starts_at,
		       s.ends_at,
		       (st.id IS NOT NULL) AS settled,
		       COALESCE(charges.total_cents, 0) AS total_cents,
		       COALESCE(charges.player_count, 0) AS player_count
		FROM sessions s
		LEFT JOIN settlements st
		       ON st.session_id = s.id AND st.reversed_at IS NULL
		LEFT JOIN (
		    SELECT settlement_id,
		           SUM(amount_cents) AS total_cents,
		           COUNT(*) AS player_count
		    FROM charge_lines
		    GROUP BY settlement_id
		) charges ON charges.settlement_id = st.id
		WHERE s.ends_at IS NOT NULL AND s.ends_at < ?
		ORDER BY s.ends_at DESC
		LIMIT ? OFFSET ?
	`, now, limit, offset).Scan(&views).Error
	if err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

// SettlementView is the full breakdown of a settled session.
type SettlementView struct {
	Session    SessionSummary       `json:"session"`
	Rates      SettlementRates      `json:"rates"`
	Bands      map[string]*BandView `json:"bands"`
	Totals     SettlementTotals     `json:"totals"`
	Lines      []ChargeLineView     `json:"lines"`
	SettledAt  time.Time            `json:"settled_at"`
	ReversedAt *time.Time           `json:"reversed_at"`
}

type SessionSummary struct {
	ID       uuid.UUID  `json:"id"`
	Title    string     `json:"title"`
	StartsAt *time.Time `json:"starts_at,omitempty"`
	EndsAt   *time.Time `json:"ends_at,omitempty"`
}

// SettlementRates is the snapshot a settlement was costed at.
type SettlementRates struct {
	BaseHours       float64 `json:"base_hours"`
	BaseRateCents   int64   `json:"base_rate_cents"`
	ExtraHours      float64 `json:"extra_hours"`
	ExtraRateCents  int64   `json:"extra_rate_cents"`
	ShuttlesPerHour float64 `json:"shuttles_per_hour"`
}

// SettlementForSession rebuilds a settled session's breakdown from what was
// stored, not from current club settings — a session viewed a year later still
// shows what it actually cost.
func (s *SettlementService) SettlementForSession(sessionID uuid.UUID) (*SettlementView, error) {
	settlement, err := s.LiveSettlementForSession(sessionID)
	if err != nil {
		return nil, err
	}
	if settlement == nil {
		return nil, nil
	}

	var session models.Session
	if err := database.DB.First(&session, "id = ?", sessionID).Error; err != nil {
		return nil, err
	}

	lines := make([]ChargeLineView, 0, len(settlement.Lines))
	userIDs := make([]uuid.UUID, 0, len(settlement.Lines))
	for _, line := range settlement.Lines {
		userIDs = append(userIDs, line.UserID)
	}

	names := map[uuid.UUID]string{}
	if len(userIDs) > 0 {
		var users []models.User
		if err := database.DB.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return nil, err
		}
		for _, u := range users {
			names[u.ID] = u.DisplayName()
		}
	}

	var charged int64
	var baseHeads, extraHeads int
	for _, line := range settlement.Lines {
		charged += line.AmountCents
		if line.InBase {
			baseHeads++
		}
		if line.InExtra {
			extraHeads++
		}
		lines = append(lines, ChargeLineView{
			UserID:      line.UserID,
			Name:        names[line.UserID],
			GuestName:   line.GuestName,
			InBase:      line.InBase,
			InExtra:     line.InExtra,
			Comped:      line.Comped,
			AmountCents: line.AmountCents,
		})
	}

	baseCourt := int64(settlement.BaseHours * float64(settlement.BaseRateCents))
	extraCourt := int64(settlement.ExtraHours * float64(settlement.ExtraRateCents))

	bands := map[string]*BandView{
		"base": {
			Hours:        settlement.BaseHours,
			CourtCents:   baseCourt,
			ShuttleUnits: settlement.BaseShuttleUnits,
			ShuttleCents: settlement.BaseShuttleCents,
			TotalCents:   baseCourt + settlement.BaseShuttleCents,
			Heads:        baseHeads,
		},
	}
	if settlement.ExtraHours > 0 {
		bands["extra"] = &BandView{
			Hours:        settlement.ExtraHours,
			CourtCents:   extraCourt,
			ShuttleUnits: settlement.ExtraShuttleUnits,
			ShuttleCents: settlement.ExtraShuttleCents,
			TotalCents:   extraCourt + settlement.ExtraShuttleCents,
			Heads:        extraHeads,
		}
	}

	consumed := settlement.CourtCents() + settlement.ShuttleCents()

	return &SettlementView{
		Session: SessionSummary{
			ID:       session.ID,
			Title:    session.Title,
			StartsAt: session.StartsAt,
			EndsAt:   session.EndsAt,
		},
		Rates: SettlementRates{
			BaseHours:       settlement.BaseHours,
			BaseRateCents:   settlement.BaseRateCents,
			ExtraHours:      settlement.ExtraHours,
			ExtraRateCents:  settlement.ExtraRateCents,
			ShuttlesPerHour: settlement.ShuttlesPerHour,
		},
		Bands: bands,
		Totals: SettlementTotals{
			CourtCents:   settlement.CourtCents(),
			ShuttleCents: settlement.ShuttleCents(),
			ShuttleUnits: settlement.BaseShuttleUnits + settlement.ExtraShuttleUnits,
			ChargedCents: charged,
			SurplusCents: charged - consumed,
		},
		Lines:      lines,
		SettledAt:  settlement.SettledAt,
		ReversedAt: settlement.ReversedAt,
	}, nil
}

// --- balance reminders ----------------------------------------------------

// BalanceNotifier is the slice of NotificationService that settlement needs.
type BalanceNotifier interface {
	SendNotification(
		ctx context.Context,
		userID uuid.UUID,
		notifType models.NotificationType,
		title, body string,
		data map[string]string,
	) error
}

// WithNotifier attaches the notifier used for balance reminders. Optional: the
// settlement service works without one, it just says nothing afterwards.
func (s *SettlementService) WithNotifier(notifier BalanceNotifier) *SettlementService {
	s.notifier = notifier
	return s
}

// notifyLowBalances tells the people who just went below the line.
//
// Reminders fire on settlement rather than on a schedule for two reasons: it is
// the moment the balance actually moved, and the message can say what the night
// cost rather than nagging in the abstract. Only that session's participants are
// considered — a member who did not play has not just been charged, so there is
// nothing new to tell them.
func (s *SettlementService) notifyLowBalances(
	ctx context.Context,
	preview *SettlementPreview,
	sessionTitle string,
) {
	if s.notifier == nil {
		return
	}

	var club models.Club
	if err := database.DB.First(&club).Error; err != nil {
		return
	}

	// One reminder per person, not one per charge line — a member who hosted a
	// guest paid twice but is still one person.
	charged := map[uuid.UUID]int64{}
	order := make([]uuid.UUID, 0, len(preview.Lines))
	for _, line := range preview.Lines {
		if line.AmountCents == 0 {
			continue
		}
		if _, seen := charged[line.UserID]; !seen {
			order = append(order, line.UserID)
		}
		charged[line.UserID] += line.AmountCents
	}

	for _, userID := range order {
		balance, err := s.ledger.BalanceOfUser(userID)
		if err != nil {
			continue
		}
		if balance >= club.LowBalanceThresholdCents {
			continue
		}

		notifType := models.NotificationBalanceLow
		title := "Your balance is running low"
		if balance < 0 {
			notifType = models.NotificationBalanceNegative
			title = "You owe the club"
		}

		body := fmt.Sprintf("%s cost you %s. Your balance is now %s.",
			sessionTitle, formatCents(charged[userID]), formatCents(balance))

		_ = s.notifier.SendNotification(ctx, userID, notifType, title, body, map[string]string{
			"balance_cents": fmt.Sprintf("%d", balance),
		})
	}
}
