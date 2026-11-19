package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services/money"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
)

const (
	SplitEqual  = "equal"
	SplitExact  = "exact"
	SplitShares = "shares"

	// The caps keep total*weight far inside int64 and stop a typo such as an
	// amount entered in cents from charging the roll thousands of dollars.
	maxSharedExpenseCents       = 1_000_000 // $10,000
	maxSharedExpenseWeight      = 100
	maxSharedExpenseDescription = 200
	maxSharedExpenseMembers     = 100
	// A backdated entry re-orders every later running balance of its members,
	// so a mistyped year must not land before the opening balances.
	maxSharedExpenseAgeDays = 365

	clubBankPayerName = "Club bank"
)

type SharedExpenseParticipant struct {
	UserID      uuid.UUID `json:"user_id"`
	AmountCents *int64    `json:"amount_cents,omitempty"` // exact only
	Weight      *int      `json:"weight,omitempty"`       // shares only
}

// SharedExpenseInput is one non-session bill, such as a meal or a coffee.
// PayerUserID nil means the club bank paid. OccurredOn is a Sydney calendar
// date, YYYY-MM-DD. RequestID identifies one submission: Confirm requires it and
// answers a repeat of the same submission with the original transaction.
type SharedExpenseInput struct {
	RequestID       uuid.UUID                  `json:"request_id,omitempty"`
	Description     string                     `json:"description"`
	OccurredOn      string                     `json:"occurred_on"`
	TotalCents      int64                      `json:"total_cents"`
	PayerUserID     *uuid.UUID                 `json:"payer_user_id"`
	SplitMethod     string                     `json:"split_method"`
	Participants    []SharedExpenseParticipant `json:"participants"`
	ExpectedPreview string                     `json:"expected_preview,omitempty"`
}

// SharedExpenseLine is one member's part. ShareCents is what they owe and is
// never negative; NetCents is the single movement their account receives, which
// for a paying member is the total less their own share.
type SharedExpenseLine struct {
	UserID     uuid.UUID `json:"user_id"`
	Name       string    `json:"name"`
	ShareCents int64     `json:"share_cents"`
	NetCents   int64     `json:"net_cents"`
}

type SharedExpensePreview struct {
	Input       SharedExpenseInput  `json:"input"`
	PayerName   string              `json:"payer_name"`
	Lines       []SharedExpenseLine `json:"lines"`
	Fingerprint string              `json:"fingerprint"`
}

// SharedExpenseService records bills that are not sessions. It owns the rules
// and hands balanced movements to LedgerService, which remains the only writer.
type SharedExpenseService struct {
	ledger   *LedgerService
	notifier BalanceNotifier
}

func NewSharedExpenseService(ledger *LedgerService, notifier BalanceNotifier) *SharedExpenseService {
	return &SharedExpenseService{ledger: ledger, notifier: notifier}
}

type sharedExpensePlan struct {
	input       SharedExpenseInput // canonical: trimmed, sorted, no fingerprint
	occurredAt  time.Time
	payerName   string
	lines       []SharedExpenseLine
	fingerprint string
}

func (s *SharedExpenseService) Preview(in SharedExpenseInput, actor *models.User) (*SharedExpensePreview, error) {
	p, err := s.plan(database.DB, in, actor)
	if err != nil {
		return nil, err
	}
	return &SharedExpensePreview{Input: p.input, PayerName: p.payerName, Lines: p.lines, Fingerprint: p.fingerprint}, nil
}

// Confirm posts the expense the admin reviewed, through the reviewed-operation
// runner. The runner takes the financial table lock, reserves the request ID,
// and SHARE-locks every member involved, so a retry of an accepted submission
// returns the original transaction instead of charging twice, and a member
// cannot be removed between the membership check and the posting. The plan is
// rebuilt inside that transaction: the figures written are exactly the figures
// the fingerprint covers.
func (s *SharedExpenseService) Confirm(in SharedExpenseInput, actor *models.User) (*models.Transaction, error) {
	if err := requireExpenseAdmin(actor); err != nil {
		return nil, err
	}
	if len(in.ExpectedPreview) != 64 {
		return nil, newLedgerError("preview_required", 409, "Review an expense preview before confirming.")
	}
	if len(in.Participants) > maxSharedExpenseMembers {
		return nil, ErrNotSettleable("Choose between 1 and 100 members to split with.")
	}
	// Checked here as well as in plan: the runner stores the submission as jsonb
	// before plan runs, and jsonb cannot hold a NUL.
	if _, err := sharedExpenseDescription(in.Description); err != nil {
		return nil, err
	}
	proposed, memberIDs, err := sharedExpenseSubmission(in)
	if err != nil {
		return nil, err
	}
	result, err := runMoneyOperation(moneyOperationRequest{
		RequestID: in.RequestID, ActorID: actor.ID, Action: models.MoneyOperationCreateSharedExpense,
		Proposed: proposed, ExpectedPreview: in.ExpectedPreview, MemberIDs: memberIDs,
	}, func(tx *gorm.DB, operationID uuid.UUID, _ moneyOperationRequest) (moneyOperationResult, error) {
		p, err := s.plan(tx, in, actor)
		if err != nil {
			return moneyOperationResult{}, err
		}
		if p.fingerprint != in.ExpectedPreview {
			return moneyOperationResult{}, newLedgerError("preview_stale", 409, "The expense changed since the preview. Review it again.")
		}
		movements, err := s.movements(tx, p)
		if err != nil {
			return moneyOperationResult{}, err
		}
		step := int16(0)
		txn, err := s.ledger.PostWithin(tx, PostInput{
			Kind:             models.TxnSharedExpense,
			MoneyOperationID: &operationID,
			OperationStep:    &step,
			Description:      p.input.Description,
			OccurredAt:       p.occurredAt,
			CreatedBy:        actor.ID,
			Movements:        movements,
		})
		if err != nil {
			return moneyOperationResult{}, err
		}
		if err := s.queueWithin(tx, txn, p); err != nil {
			return moneyOperationResult{}, err
		}
		body, err := json.Marshal(txn)
		if err != nil {
			return moneyOperationResult{}, err
		}
		return moneyOperationResult{Status: 201, Body: body}, nil
	})
	if err != nil {
		return nil, err
	}
	var txn models.Transaction
	if err := json.Unmarshal(result.Body, &txn); err != nil {
		return nil, err
	}
	return &txn, nil
}

// sharedExpenseSubmission gives the runner the contents that one request ID is
// bound to, in a form that does not depend on the order the form sent, and the
// members to lock. It validates nothing: plan does that under the locks.
func sharedExpenseSubmission(in SharedExpenseInput) (json.RawMessage, []uuid.UUID, error) {
	participants := append([]SharedExpenseParticipant(nil), in.Participants...)
	sort.SliceStable(participants, func(i, j int) bool { return participants[i].UserID.String() < participants[j].UserID.String() })
	memberIDs := make([]uuid.UUID, 0, len(participants)+1)
	for _, p := range participants {
		memberIDs = append(memberIDs, p.UserID)
	}
	if in.PayerUserID != nil {
		memberIDs = append(memberIDs, *in.PayerUserID)
	}
	proposed, err := json.Marshal(SharedExpenseInput{
		Description: strings.TrimSpace(in.Description), OccurredOn: in.OccurredOn, TotalCents: in.TotalCents,
		PayerUserID: in.PayerUserID, SplitMethod: in.SplitMethod, Participants: participants,
	})
	return proposed, memberIDs, err
}

func (s *SharedExpenseService) plan(db *gorm.DB, in SharedExpenseInput, actor *models.User) (*sharedExpensePlan, error) {
	if err := requireExpenseAdmin(actor); err != nil {
		return nil, err
	}
	description, err := sharedExpenseDescription(in.Description)
	if err != nil {
		return nil, err
	}
	if in.TotalCents <= 0 || in.TotalCents > maxSharedExpenseCents {
		return nil, ErrNotSettleable("Enter a total between $0.01 and $10,000.")
	}
	now := utils.NowInSydney()
	var day time.Time
	day, err = time.ParseInLocation("2006-01-02", in.OccurredOn, now.Location())
	if err != nil {
		return nil, ErrNotSettleable("Enter the date of the expense.")
	}
	if day.After(now) {
		return nil, ErrNotSettleable("The expense date cannot be in the future.")
	}
	// Midnight to midnight: the limit must not move with the time of day.
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if day.Before(today.AddDate(0, 0, -maxSharedExpenseAgeDays)) {
		return nil, ErrNotSettleable("The expense date cannot be more than a year ago.")
	}
	occurredAt := day.Add(12 * time.Hour)
	if day.Format("2006-01-02") == now.Format("2006-01-02") {
		occurredAt = now
	}

	if len(in.Participants) == 0 || len(in.Participants) > maxSharedExpenseMembers {
		return nil, ErrNotSettleable("Choose between 1 and 100 members to split with.")
	}
	// Sorting first makes the result independent of the order the form sent.
	participants := append([]SharedExpenseParticipant(nil), in.Participants...)
	sort.Slice(participants, func(i, j int) bool { return participants[i].UserID.String() < participants[j].UserID.String() })
	ids := make([]uuid.UUID, len(participants))
	for i, p := range participants {
		if p.UserID == uuid.Nil || (i > 0 && participants[i-1].UserID == p.UserID) {
			return nil, ErrNotSettleable("Each member can appear only once.")
		}
		ids[i] = p.UserID
	}
	if in.PayerUserID != nil && *in.PayerUserID == uuid.Nil {
		return nil, ErrNotSettleable("Choose who paid.")
	}

	shares, err := sharedExpenseShares(in.SplitMethod, in.TotalCents, participants, ids, sharedExpenseSeed(in.OccurredOn, description, in.TotalCents))
	if err != nil {
		return nil, err
	}

	// A paying member who is not splitting still needs a line for their credit.
	memberIDs := append([]uuid.UUID(nil), ids...)
	net := make(map[uuid.UUID]int64, len(ids)+1)
	for id, share := range shares {
		net[id] = -share
	}
	if in.PayerUserID != nil {
		if _, splitting := shares[*in.PayerUserID]; !splitting {
			memberIDs = append(memberIDs, *in.PayerUserID)
		} else if len(ids) == 1 {
			return nil, ErrNotSettleable("Nothing to split: the payer is the only participant.")
		}
		net[*in.PayerUserID] += in.TotalCents
	}

	var users []models.User
	if err := db.Where("id IN ? AND membership_status = ?", memberIDs, models.MembershipApproved).Find(&users).Error; err != nil {
		return nil, err
	}
	if len(users) != len(memberIDs) {
		return nil, ErrNotSettleable("Each participant and the payer must be an approved club member.")
	}

	position, err := s.ledger.Position()
	if err != nil {
		return nil, err
	}
	if position.AssetsPending {
		return nil, ErrNotSettleable("Confirm club assets before recording expenses.")
	}

	payerName := clubBankPayerName
	lines := make([]SharedExpenseLine, 0, len(users))
	for i := range users {
		u := &users[i]
		if in.PayerUserID != nil && u.ID == *in.PayerUserID {
			payerName = u.DisplayName()
		}
		lines = append(lines, SharedExpenseLine{UserID: u.ID, Name: u.DisplayName(), ShareCents: shares[u.ID], NetCents: net[u.ID]})
	}
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].Name != lines[j].Name {
			return lines[i].Name < lines[j].Name
		}
		return lines[i].UserID.String() < lines[j].UserID.String()
	})

	canonical := SharedExpenseInput{
		Description: description, OccurredOn: in.OccurredOn, TotalCents: in.TotalCents,
		PayerUserID: in.PayerUserID, SplitMethod: in.SplitMethod, Participants: participants,
	}
	return &sharedExpensePlan{
		input: canonical, occurredAt: occurredAt, payerName: payerName, lines: lines,
		fingerprint: sharedExpenseFingerprint(canonical, lines),
	}, nil
}

// sharedExpenseDescription trims a description and refuses one that is empty,
// too long, or holds a control character. A NUL would preview cleanly and then
// fail to save, because jsonb cannot store it.
func sharedExpenseDescription(raw string) (string, error) {
	description := strings.TrimSpace(raw)
	if description == "" || utf8.RuneCountInString(description) > maxSharedExpenseDescription {
		return "", ErrNotSettleable("Enter a description of up to 200 characters.")
	}
	if !utf8.ValidString(description) || strings.ContainsFunc(description, unicode.IsControl) {
		return "", ErrNotSettleable("The description contains characters that cannot be saved.")
	}
	return description, nil
}

// sharedExpenseShares returns what each participant owes. Every share is
// positive and the shares sum to the total, or the expense is refused.
func sharedExpenseShares(method string, total int64, participants []SharedExpenseParticipant, ids []uuid.UUID, seed uuid.UUID) (map[uuid.UUID]int64, error) {
	var split []money.Share
	var err error
	switch method {
	case SplitEqual:
		for _, p := range participants {
			if p.AmountCents != nil || p.Weight != nil {
				return nil, ErrNotSettleable("An equal split takes no amounts or shares.")
			}
		}
		split, err = money.SplitLargestRemainder(total, ids, seed)
	case SplitExact:
		amounts := make([]int64, len(participants))
		for i, p := range participants {
			if p.Weight != nil || p.AmountCents == nil {
				return nil, ErrNotSettleable("Enter an amount for each person.")
			}
			amounts[i] = *p.AmountCents
			split = append(split, money.Share{ParticipantID: p.UserID, AmountCents: *p.AmountCents})
		}
		if money.CheckExact(total, amounts) != nil {
			return nil, ErrNotSettleable("Each amount must be more than zero and the amounts must add up to the total.")
		}
	case SplitShares:
		weights := make([]int, len(participants))
		for i, p := range participants {
			if p.AmountCents != nil || p.Weight == nil || *p.Weight < 1 || *p.Weight > maxSharedExpenseWeight {
				return nil, ErrNotSettleable("Enter a whole number of shares from 1 to 100 for each person.")
			}
			weights[i] = *p.Weight
		}
		split, err = money.SplitByWeights(total, ids, weights, seed)
	default:
		return nil, ErrNotSettleable("Choose how to split the expense.")
	}
	if err != nil {
		return nil, err
	}
	if money.SumShares(split) != total {
		return nil, errors.New("shared expense: shares do not sum to the total")
	}
	shares := make(map[uuid.UUID]int64, len(split))
	for _, share := range split {
		if share.AmountCents <= 0 {
			return nil, ErrNotSettleable("The total is too small to split between this many people.")
		}
		shares[share.ParticipantID] = share.AmountCents
	}
	return shares, nil
}

// sharedExpenseSeed decides who carries a spare cent. It is fixed by the bill
// itself, so the preview and its confirmation always agree.
func sharedExpenseSeed(occurredOn, description string, total int64) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(occurredOn+"|"+description+"|"+strconv.FormatInt(total, 10)))
}

// sharedExpenseFingerprint binds the reviewed shares to their inputs. It is a
// concurrency token, not an authorization token; routes enforce access.
func sharedExpenseFingerprint(in SharedExpenseInput, lines []SharedExpenseLine) string {
	type line struct {
		ID         uuid.UUID
		Share, Net int64
	}
	ordered := make([]line, len(lines))
	for i, l := range lines {
		ordered[i] = line{l.UserID, l.ShareCents, l.NetCents}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID.String() < ordered[j].ID.String() })
	data, _ := json.Marshal(struct {
		Description, OccurredOn string
		Total                   int64
		Payer                   *uuid.UUID
		Method                  string
		Lines                   []line
	}{in.Description, in.OccurredOn, in.TotalCents, in.PayerUserID, in.SplitMethod, ordered})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *SharedExpenseService) movements(tx *gorm.DB, p *sharedExpensePlan) ([]Movement, error) {
	movements := make([]Movement, 0, len(p.lines)+1)
	if p.input.PayerUserID == nil {
		bank, err := s.ledger.ClubAccountID(tx, models.AccountKindBank)
		if err != nil {
			return nil, err
		}
		movements = append(movements, Movement{AccountID: bank, AmountCents: -p.input.TotalCents})
	}
	for _, line := range p.lines {
		if line.NetCents == 0 {
			continue // paid exactly their own share: nothing moves
		}
		account, err := s.ledger.PlayerAccountID(line.UserID)
		if err != nil {
			return nil, err
		}
		movements = append(movements, Movement{AccountID: account, AmountCents: line.NetCents})
	}
	return movements, nil
}

// queueWithin captures one notification per affected member under the money
// transaction, so a rolled-back expense notifies nobody.
func (s *SharedExpenseService) queueWithin(tx *gorm.DB, txn *models.Transaction, p *sharedExpensePlan) error {
	queue := s.notifier
	if queue == nil {
		queue = &NotificationService{disabled: true}
	}
	lines := append([]SharedExpenseLine(nil), p.lines...)
	sort.Slice(lines, func(i, j int) bool { return lines[i].UserID.String() < lines[j].UserID.String() })
	for _, line := range lines {
		if line.NetCents == 0 {
			continue
		}
		balance, err := balanceWithin(tx, line.UserID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		body := fmt.Sprintf("Charged %s for %s. Balance after this split: %s.", formatCents(-line.NetCents), p.input.Description, formatCents(balance))
		if line.NetCents > 0 {
			body = fmt.Sprintf("Credited %s for paying %s. Balance now: %s.", formatCents(line.NetCents), p.input.Description, formatCents(balance))
		}
		if warning := BalanceWarning(balance); warning != "" {
			body += " " + warning
		}
		kind := models.NotificationSharedExpense
		notification := models.Notification{
			UserID: line.UserID, NotificationType: kind, Title: "Shared expense", Body: body,
			Data: mustNotificationJSON(map[string]string{
				"type": string(kind), "source": "shared_expense", "transaction_id": txn.ID.String(),
				"amount_cents":        strconv.FormatInt(line.NetCents, 10),
				"balance_after_cents": strconv.FormatInt(balance, 10),
				"balance_band":        BalanceNotificationBand(balance),
				"checked_at":          now.Format(time.RFC3339Nano),
				"destination_path":    "/money",
			}),
		}
		key := sharedExpenseReceiptKey(txn.ID) + line.UserID.String()
		if _, err := queue.QueueNotificationWithin(tx, &notification, key, now, now.Add(24*time.Hour)); err != nil {
			return err
		}
	}
	return nil
}

// queueSharedExpenseReversalWithin tells each member that a shared expense was
// undone. The original notice may already be in their history, where it cannot
// be withdrawn, so without this they would still read of a charge that no
// longer exists.
func (s *LedgerService) queueSharedExpenseReversalWithin(tx *gorm.DB, original, reversal *models.Transaction) error {
	queue := s.notifier
	if queue == nil {
		queue = &NotificationService{disabled: true}
	}
	type effect struct {
		UserID      uuid.UUID
		AmountCents int64
	}
	var effects []effect
	if err := tx.Raw(`SELECT a.user_id, SUM(e.amount_cents) AS amount_cents
		FROM ledger_entries e JOIN accounts a ON a.id = e.account_id
		WHERE e.transaction_id = ? AND a.kind = 'player' AND a.user_id IS NOT NULL
		GROUP BY a.user_id ORDER BY a.user_id`, original.ID).Scan(&effects).Error; err != nil {
		return err
	}
	for _, e := range effects {
		if e.AmountCents == 0 {
			continue
		}
		balance, err := balanceWithin(tx, e.UserID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		body := fmt.Sprintf("The %s charge for %s was reversed. Balance after this correction: %s.", formatCents(-e.AmountCents), original.Description, formatCents(balance))
		if e.AmountCents > 0 {
			body = fmt.Sprintf("The %s credit for paying %s was reversed. Balance after this correction: %s.", formatCents(e.AmountCents), original.Description, formatCents(balance))
		}
		if warning := BalanceWarning(balance); warning != "" {
			body += " " + warning
		}
		kind := models.NotificationSharedExpense
		notification := models.Notification{
			UserID: e.UserID, NotificationType: kind, Title: "Shared expense reversed", Body: body,
			Data: mustNotificationJSON(map[string]string{
				"type": string(kind), "source": "shared_expense_reversal", "transaction_id": original.ID.String(),
				"reversal_transaction_id": reversal.ID.String(),
				"amount_cents":            strconv.FormatInt(-e.AmountCents, 10),
				"balance_after_cents":     strconv.FormatInt(balance, 10),
				"balance_band":            BalanceNotificationBand(balance),
				"checked_at":              now.Format(time.RFC3339Nano),
				"destination_path":        "/money",
			}),
		}
		key := fmt.Sprintf("shared-expense-reversal:%s:%s", original.ID, e.UserID)
		if _, err := queue.QueueNotificationWithin(tx, &notification, key, now, now.Add(24*time.Hour)); err != nil {
			return err
		}
	}
	return nil
}
