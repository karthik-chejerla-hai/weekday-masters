package services

import (
	"bytes"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
)

// An activity is either one account movement or one complete game. A game has
// no single member balance: its balances belong to the shares inside it.
type LedgerActivityView struct {
	ID         uuid.UUID        `json:"id"`
	OccurredAt time.Time        `json:"occurred_at"`
	Entry      *LedgerEntryView `json:"entry,omitempty"`
	Game       *LedgerGameView  `json:"game,omitempty"`
	order      ledgerOrder
}

type LedgerGameView struct {
	SessionID         uuid.UUID         `json:"session_id"`
	Title             string            `json:"title"`
	PlayedDate        string            `json:"played_date"`
	DateBasis         string            `json:"date_basis"`
	Source            string            `json:"source,omitempty"`
	TotalChargedCents int64             `json:"total_charged_cents"`
	Reversed          bool              `json:"reversed"`
	SourceCount       int               `json:"source_count"`
	Shares            []LedgerGameShare `json:"shares"`
}

type LedgerGameShare struct {
	ID                uuid.UUID  `json:"id"`
	UserID            *uuid.UUID `json:"user_id,omitempty"`
	MemberName        string     `json:"member_name"`
	Inactive          bool       `json:"inactive"`
	ChargeCents       int64      `json:"charge_cents"`
	PaidCents         int64      `json:"paid_cents"`
	AmountCents       int64      `json:"amount_cents"`
	BalanceAfterCents int64      `json:"balance_after_cents"`
	GuestNames        []string   `json:"guest_names,omitempty"`
	accountID         uuid.UUID
}

type ledgerOrder struct {
	occurredAt time.Time
	createdAt  time.Time
	id         uuid.UUID
}

func (a ledgerOrder) before(b ledgerOrder) bool {
	if !a.occurredAt.Equal(b.occurredAt) {
		return a.occurredAt.Before(b.occurredAt)
	}
	if !a.createdAt.Equal(b.createdAt) {
		return a.createdAt.Before(b.createdAt)
	}
	return bytes.Compare(a.id[:], b.id[:]) < 0
}

func entryOrder(e LedgerEntryView) ledgerOrder { return ledgerOrder{e.OccurredAt, e.CreatedAt, e.ID} }

type gameActivityBuilder struct {
	activity LedgerActivityView
	shares   map[uuid.UUID]*LedgerGameShare
}

// Activity groups before filtering and paging. Reading a consistent snapshot
// keeps source shares, account balances and page totals in step during writes.
func (s *LedgerService) Activity(filter LedgerHistoryFilter) ([]LedgerActivityView, int64, error) {
	if filter.TopupsOnly {
		entries, total, err := s.Entries(filter)
		if err != nil {
			return nil, 0, err
		}
		views := make([]LedgerActivityView, 0, len(entries))
		for i := range entries {
			views = append(views, LedgerActivityView{ID: entries[i].ID, OccurredAt: entries[i].OccurredAt, Entry: &entries[i]})
		}
		return views, total, nil
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	views := []LedgerActivityView{}
	var total int64
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		all, err := readLedgerActivities(tx)
		if err != nil {
			return err
		}
		for _, item := range all {
			if filter.UserID != nil {
				mine := item.Entry != nil && item.Entry.UserID != nil && *item.Entry.UserID == *filter.UserID
				if item.Game != nil {
					for _, share := range item.Game.Shares {
						if share.UserID != nil && *share.UserID == *filter.UserID {
							mine = true
							break
						}
					}
				}
				if !mine {
					continue
				}
			}
			if total >= int64(filter.Offset) && len(views) < filter.Limit {
				views = append(views, item)
			}
			total++
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return views, total, err
}

func readLedgerActivities(tx *gorm.DB) ([]LedgerActivityView, error) {
	var history []LedgerEntryView
	if err := tx.Raw(ledgerHistorySQL + "SELECT * FROM history ORDER BY occurred_at, created_at, id").Scan(&history).Error; err != nil {
		return nil, err
	}
	byAccount := map[uuid.UUID][]LedgerEntryView{}
	transactionEnd := map[uuid.UUID]ledgerOrder{}
	for _, e := range history {
		byAccount[e.AccountID] = append(byAccount[e.AccountID], e)
		transactionEnd[e.TransactionID] = entryOrder(e)
	}
	represented := map[uuid.UUID]bool{}
	games := map[string]*gameActivityBuilder{}
	attach := func(g *gameActivityBuilder, t *models.Transaction) {
		represented[t.ID] = true
		end, ok := transactionEnd[t.ID]
		if !ok {
			end = ledgerOrder{t.OccurredAt, t.CreatedAt, t.ID}
		}
		if g.activity.order.before(end) {
			g.activity.order = end
			g.activity.OccurredAt = t.OccurredAt
		}
	}

	// Imports retain actual gross charges and paid-on-behalf amounts. Net ledger
	// credits alone cannot tell us a payer's own share of the expense.
	var records []models.SplitwiseRecord
	if err := tx.Preload("Transaction").Preload("Changes.Participant.User").Where("is_session = true").Order("row_number, id").Find(&records).Error; err != nil {
		return nil, err
	}
	for _, r := range records {
		if r.Transaction == nil {
			return nil, errors.New("imported game transaction missing")
		}
		key := r.ImportID.String() + "/" + r.PlayedDate.Format("2006-01-02")
		g := games[key]
		if g == nil {
			g = &gameActivityBuilder{activity: LedgerActivityView{ID: r.ID, Game: &LedgerGameView{SessionID: r.ID, Title: r.Description, PlayedDate: r.PlayedDate.Format("2006-01-02"), DateBasis: r.DateBasis, Source: "splitwise"}}, shares: map[uuid.UUID]*LedgerGameShare{}}
			games[key] = g
		}
		attach(g, r.Transaction)
		g.activity.Game.SourceCount++
		for _, c := range r.Changes {
			if c.ChargeCents == nil || (*c.ChargeCents == 0 && c.PaidCents == 0) {
				continue
			}
			p := c.Participant
			if p == nil {
				return nil, errors.New("imported game participant missing")
			}
			share := g.shares[p.AccountID]
			if share == nil {
				share = &LedgerGameShare{ID: p.AccountID, accountID: p.AccountID, UserID: p.UserID, MemberName: strings.TrimSuffix(p.SourceName, " (removed)"), Inactive: p.User == nil}
				if p.User != nil {
					share.MemberName = p.User.DisplayName()
					share.Inactive = p.User.MembershipStatus == models.MembershipRemoved
				}
				g.shares[p.AccountID] = share
			}
			share.ChargeCents += *c.ChargeCents
			share.PaidCents += c.PaidCents
			share.AmountCents += c.NetCents
		}
	}

	// Native charge lines include comped members and guests even when no ledger
	// entry was needed. Combine guest charges into their host's account share.
	var settlements []models.Settlement
	if err := tx.Preload("Session").Preload("Lines.User").Find(&settlements).Error; err != nil {
		return nil, err
	}
	if len(settlements) > 0 {
		var accounts []models.Account
		if err := tx.Where("kind = ?", models.AccountKindPlayer).Find(&accounts).Error; err != nil {
			return nil, err
		}
		accountByUser := map[uuid.UUID]uuid.UUID{}
		for _, a := range accounts {
			if a.UserID != nil {
				accountByUser[*a.UserID] = a.ID
			}
		}
		transactionIDs := make([]uuid.UUID, 0, len(settlements))
		for _, record := range settlements {
			transactionIDs = append(transactionIDs, record.TransactionID)
		}
		var transactions []models.Transaction
		if err := tx.Where("id IN ?", transactionIDs).Find(&transactions).Error; err != nil {
			return nil, err
		}
		byID := map[uuid.UUID]*models.Transaction{}
		for i := range transactions {
			byID[transactions[i].ID] = &transactions[i]
		}
		for _, record := range settlements {
			t := byID[record.TransactionID]
			if t == nil || record.Session == nil {
				return nil, errors.New("settled game transaction or session missing")
			}
			g := &gameActivityBuilder{activity: LedgerActivityView{ID: t.ID, Game: &LedgerGameView{SessionID: record.SessionID, Title: record.Session.Title, PlayedDate: record.Session.SessionDate.Format("2006-01-02"), DateBasis: "title", Reversed: record.ReversedAt != nil, SourceCount: 1}}, shares: map[uuid.UUID]*LedgerGameShare{}}
			games[t.ID.String()] = g
			attach(g, t)
			for _, line := range record.Lines {
				if line.User == nil {
					return nil, errors.New("settled game member missing")
				}
				share := g.shares[line.UserID]
				if share == nil {
					share = &LedgerGameShare{ID: line.UserID, UserID: &line.UserID, accountID: accountByUser[line.UserID], MemberName: line.User.DisplayName(), Inactive: line.User.MembershipStatus == models.MembershipRemoved}
					g.shares[line.UserID] = share
				}
				share.ChargeCents += line.AmountCents
				share.AmountCents -= line.AmountCents
				if line.GuestName != "" {
					share.GuestNames = append(share.GuestNames, line.GuestName)
				}
			}
		}
	}

	activities := make([]LedgerActivityView, 0, len(games)+len(history))
	for _, g := range games {
		game := g.activity.Game
		game.Shares = make([]LedgerGameShare, 0, len(g.shares))
		for _, share := range g.shares {
			entries := byAccount[share.accountID]
			// A group's last posting is its accounting boundary. This also includes
			// unrelated intervening top-ups between regular and extra-hour imports.
			i := sort.Search(len(entries), func(i int) bool { return g.activity.order.before(entryOrder(entries[i])) })
			if i > 0 {
				share.BalanceAfterCents = entries[i-1].BalanceAfterCents
			}
			game.TotalChargedCents += share.ChargeCents
			sort.Strings(share.GuestNames)
			game.Shares = append(game.Shares, *share)
		}
		sort.Slice(game.Shares, func(i, j int) bool {
			a, b := game.Shares[i], game.Shares[j]
			if a.MemberName != b.MemberName {
				return a.MemberName < b.MemberName
			}
			return bytes.Compare(a.ID[:], b.ID[:]) < 0
		})
		activities = append(activities, g.activity)
	}
	for i := range history {
		e := &history[i]
		if represented[e.TransactionID] {
			continue
		}
		activities = append(activities, LedgerActivityView{ID: e.ID, OccurredAt: e.OccurredAt, Entry: e, order: entryOrder(*e)})
	}
	sort.Slice(activities, func(i, j int) bool { return activities[j].order.before(activities[i].order) })
	return activities, nil
}
