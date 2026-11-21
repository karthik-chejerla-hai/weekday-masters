package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
)

// These tests cover the two rules that keep the settle route honest:
//
//   - it never posts from a request it could not read in full, and
//   - it never posts an amount other than the one that was reviewed.

// unreviewedFingerprint is well-formed but matches no preview. It gets a request
// past "is a fingerprint present" so a test can reach the check behind it.
var unreviewedFingerprint = strings.Repeat("0", 64)

// postRaw sends body verbatim, for requests json.Marshal could never produce.
func (h *harness) postRaw(path, body string) *response {
	h.t.Helper()

	req, err := http.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	if err != nil {
		h.t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return &response{t: h.t, Code: w.Code, Body: w.Body.Bytes()}
}

func settlePath(sessionID uuid.UUID) string {
	return fmt.Sprintf("/api/admin/sessions/%s/settle", sessionID)
}

func previewPath(sessionID uuid.UUID) string {
	return fmt.Sprintf("/api/admin/sessions/%s/settlement/preview", sessionID)
}

// previewFingerprint costs body and returns the fingerprint of what came back.
func (h *harness) previewFingerprint(admin *models.User, sessionID uuid.UUID, body map[string]any) string {
	h.t.Helper()

	var preview struct {
		Fingerprint string `json:"fingerprint"`
	}
	h.as(admin).post(previewPath(sessionID), body).expect(http.StatusOK).decode(&preview)
	if len(preview.Fingerprint) != 64 {
		h.t.Fatalf("preview fingerprint = %q, want 64 hex characters", preview.Fingerprint)
	}
	return preview.Fingerprint
}

// settleReviewed settles the way the form does: preview, then confirm with that
// preview's fingerprint.
func (h *harness) settleReviewed(admin *models.User, sessionID uuid.UUID, body map[string]any) *response {
	h.t.Helper()

	reviewed := map[string]any{}
	for key, value := range body {
		reviewed[key] = value
	}
	reviewed["expected_preview"] = h.previewFingerprint(admin, sessionID, body)
	return h.as(admin).post(settlePath(sessionID), reviewed)
}

// moneyState is everything a refused request must leave alone.
type moneyState struct {
	Transactions int64
	Entries      int64
	Settlements  int64
	ChargeLines  int64
	StockUnits   int
	StockCents   int64
}

func readMoneyState(t *testing.T) moneyState {
	t.Helper()

	var state moneyState
	for model, into := range map[any]*int64{
		&models.Transaction{}: &state.Transactions,
		&models.LedgerEntry{}: &state.Entries,
		&models.Settlement{}:  &state.Settlements,
		&models.ChargeLine{}:  &state.ChargeLines,
	} {
		if err := database.DB.Model(model).Count(into).Error; err != nil {
			t.Fatalf("failed to count %T: %v", model, err)
		}
	}
	stock, err := services.NewLedgerService().StockPosition(nil)
	if err != nil {
		t.Fatalf("failed to read stock: %v", err)
	}
	state.StockUnits, state.StockCents = stock.Units, stock.ValueCents
	return state
}

func expectMoneyUnchanged(t *testing.T, before moneyState) {
	t.Helper()
	if after := readMoneyState(t); after != before {
		t.Errorf("a refused request moved money:\nbefore %+v\nafter  %+v", before, after)
	}
}

// --- unreadable input -----------------------------------------------------

// The reproduction: {"extra_hours":"invalid"} used to be bound, fail, have its
// error discarded, and settle the session on defaults with a 201.
func TestSettleRejectsInputItCannotReadAndPostsNothing(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := pastSession(t, h, admin)

	line := fmt.Sprintf(`{"user_id":%q,"in_base":true}`, player.ID)
	fingerprint := h.previewFingerprint(admin, session.ID, map[string]any{
		"lines": []map[string]any{{"user_id": player.ID.String(), "in_base": true}},
	})
	// A request that would settle if it were sent on its own, so each case below
	// fails for the one reason it names and not for a missing fingerprint.
	valid := fmt.Sprintf(`{"lines":[%s],"expected_preview":%q}`, line, fingerprint)

	cases := map[string]string{
		"a string where a number belongs":   fmt.Sprintf(`{"extra_hours":"invalid","lines":[%s],"expected_preview":%q}`, line, fingerprint),
		"a number where the lines belong":   fmt.Sprintf(`{"lines":7,"expected_preview":%q}`, fingerprint),
		"a string where a flag belongs":     fmt.Sprintf(`{"lines":[{"user_id":%q,"in_base":"yes"}],"expected_preview":%q}`, player.ID, fingerprint),
		"a number where the review belongs": fmt.Sprintf(`{"lines":[%s],"expected_preview":12}`, line),
		"an array instead of an object":     `[]`,
		"broken JSON":                       fmt.Sprintf(`{"lines":[%s],"expected_preview":%q`, line, fingerprint),
		"not JSON at all":                   `settle it`,
		"a second JSON value":               valid + valid,
		"trailing text":                     valid + ` trailing`,
		"no review data":                    fmt.Sprintf(`{"lines":[%s]}`, line),
		"an empty review":                   fmt.Sprintf(`{"lines":[%s],"expected_preview":""}`, line),
		"a null review":                     fmt.Sprintf(`{"lines":[%s],"expected_preview":null}`, line),
		"a truncated review":                fmt.Sprintf(`{"lines":[%s],"expected_preview":%q}`, line, fingerprint[:63]),
		"an empty object":                   `{}`,
		"an empty body":                     ``,
		"a whitespace body":                 "  \n",
		"null":                              `null`,
	}

	before := readMoneyState(t)
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			h.t = t
			failure := h.as(admin).postRaw(settlePath(session.ID), body).
				expect(http.StatusBadRequest).ledgerError()
			if failure.Code != "bad_request" {
				t.Errorf("code = %q, want bad_request", failure.Code)
			}
			expectMoneyUnchanged(t, before)
		})
	}
	h.t = t

	// The control: the request the cases above were built from does settle, so
	// they were refused for what was wrong with them.
	h.as(admin).postRaw(settlePath(session.ID), valid).expect(http.StatusCreated)
}

// An empty body means "cost the default", and only an empty body means that.
func TestPreviewSettlementTreatsOnlyAnEmptyBodyAsTheDefault(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := pastSession(t, h, admin)
	if err := database.DB.Create(&models.RSVP{
		SessionID: session.ID, UserID: player.ID, Status: models.RSVPStatusIn,
	}).Error; err != nil {
		t.Fatal(err)
	}

	for name, body := range map[string]string{
		"no body":         ``,
		"whitespace only": " \n\t",
		"an empty object": `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			h.t = t
			var preview previewBody
			h.as(admin).postRaw(previewPath(session.ID), body).expect(http.StatusOK).decode(&preview)
			if len(preview.Lines) != 1 || preview.Totals.ChargedCents != 10167 {
				t.Errorf("default preview = %d lines charging %d, want the one RSVP charged 10167",
					len(preview.Lines), preview.Totals.ChargedCents)
			}
		})
	}

	before := readMoneyState(t)
	for name, body := range map[string]string{
		"a string where a number belongs": `{"extra_hours":"invalid"}`,
		"a number where the lines belong": `{"lines":7}`,
		"broken JSON":                     `{"extra_hours":1`,
		"not JSON at all":                 `preview it`,
		"a second JSON value":             `{}{}`,
		"trailing text":                   `{} trailing`,
	} {
		t.Run(name, func(t *testing.T) {
			h.t = t
			h.as(admin).postRaw(previewPath(session.ID), body).expect(http.StatusBadRequest)
			expectMoneyUnchanged(t, before)
		})
	}
	h.t = t
}

// Both reverse routes take an optional description. A body that is present but
// broken is not "no description" — it is a request the server could not read.
func TestReverseRoutesRejectBrokenJSONAndPostNoReversal(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := pastSession(t, h, admin)

	var created struct {
		Settlement models.Settlement `json:"settlement"`
	}
	h.settleReviewed(admin, session.ID, map[string]any{
		"lines": []map[string]any{{"user_id": player.ID.String(), "in_base": true}},
	}).expect(http.StatusCreated).decode(&created)

	routes := map[string]string{
		"settlement":  fmt.Sprintf("/api/admin/settlements/%s/reverse", created.Settlement.ID),
		"transaction": fmt.Sprintf("/api/admin/transactions/%s/reverse", created.Settlement.TransactionID),
	}
	bodies := map[string]string{
		"broken JSON":                        `{"description":"wrong players"`,
		"not JSON at all":                    `undo it`,
		"a number where the reason belongs":  `{"description":7}`,
		"a second JSON value":                `{"description":"a"}{"description":"b"}`,
		"trailing text":                      `{"description":"a"} trailing`,
		"an array where the object belongs":  `["wrong players"]`,
		"a string where the object belongs":  `"wrong players"`,
		"an object where the reason belongs": `{"description":{"text":"a"}}`,
	}

	before := readMoneyState(t)
	for route, path := range routes {
		for name, body := range bodies {
			t.Run(route+"/"+name, func(t *testing.T) {
				h.t = t
				h.as(admin).postRaw(path, body).expect(http.StatusBadRequest)
				expectMoneyUnchanged(t, before)

				var settlement models.Settlement
				if err := database.DB.First(&settlement, "id = ?", created.Settlement.ID).Error; err != nil {
					t.Fatal(err)
				}
				if settlement.ReversedAt != nil {
					t.Fatal("a broken request reversed the settlement")
				}
			})
		}
	}
	h.t = t

	// No body at all stays valid, on both routes.
	h.as(admin).postRaw(routes["settlement"], ``).expect(http.StatusCreated)
	h.settleReviewed(admin, session.ID, map[string]any{
		"lines": []map[string]any{{"user_id": player.ID.String(), "in_base": true}},
	}).expect(http.StatusCreated).decode(&created)
	h.as(admin).postRaw(fmt.Sprintf("/api/admin/transactions/%s/reverse", created.Settlement.TransactionID), " \n").
		expect(http.StatusCreated)
}

// --- the reviewed amount --------------------------------------------------

func expectPreviewChanged(t *testing.T, r *response) {
	t.Helper()
	failure := r.expect(http.StatusConflict).ledgerError()
	if failure.Code != "preview_changed" {
		t.Errorf("code = %q, want preview_changed", failure.Code)
	}
}

// The reproduction: a tube recorded between preview and confirmation changes the
// blended shuttle value, so the night that was reviewed at one total would have
// posted at another.
func TestSettleRefusesWhenStockChangedAfterThePreview(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := pastSession(t, h, admin)

	body := map[string]any{
		"lines": []map[string]any{{"user_id": player.ID.String(), "in_base": true}},
	}
	var reviewed struct {
		Fingerprint string `json:"fingerprint"`
		Totals      struct {
			ChargedCents int64 `json:"charged_cents"`
		} `json:"totals"`
	}
	h.as(admin).post(previewPath(session.ID), body).expect(http.StatusOK).decode(&reviewed)
	if reviewed.Totals.ChargedCents != 10167 {
		t.Fatalf("reviewed charge = %d, want 10167", reviewed.Totals.ChargedCents)
	}

	// A dearer tube: the same ten shuttles now cost more.
	h.as(admin).post("/api/admin/transactions/shuttle-purchase", map[string]any{
		"units": 12, "amount_cents": 20000,
	}).expect(http.StatusCreated)

	var now previewBody
	h.as(admin).post(previewPath(session.ID), body).expect(http.StatusOK).decode(&now)
	if now.Totals.ChargedCents == reviewed.Totals.ChargedCents {
		t.Fatal("the purchase did not change the costing, so this test proves nothing")
	}

	before := readMoneyState(t)
	body["expected_preview"] = reviewed.Fingerprint
	expectPreviewChanged(t, h.as(admin).post(settlePath(session.ID), body))
	expectMoneyUnchanged(t, before)

	var balance struct {
		BalanceCents int64 `json:"balance_cents"`
	}
	h.as(player).get("/api/accounts/me").expect(http.StatusOK).decode(&balance)
	if balance.BalanceCents != 0 {
		t.Errorf("player balance = %d, want 0: nothing was reviewed at the new price", balance.BalanceCents)
	}
}

// Stock that changes without changing tonight's price is still a different bag
// from the one reviewed: the preview shows what is left afterwards too.
func TestSettleRefusesWhenStockChangedAtTheSameUnitPrice(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := pastSession(t, h, admin)

	body := map[string]any{
		"lines": []map[string]any{{"user_id": player.ID.String(), "in_base": true}},
	}
	fingerprint := h.previewFingerprint(admin, session.ID, body)

	// pastSession stocks 24 for $100; another 24 for $100 keeps the blend.
	h.as(admin).post("/api/admin/transactions/shuttle-purchase", map[string]any{
		"units": 24, "amount_cents": 10000,
	}).expect(http.StatusCreated)

	before := readMoneyState(t)
	body["expected_preview"] = fingerprint
	expectPreviewChanged(t, h.as(admin).post(settlePath(session.ID), body))
	expectMoneyUnchanged(t, before)
}

// A request that names no rates is costed on club settings, so changing those
// settings changes what it would post.
func TestSettleRefusesWhenRatesChangedAfterThePreview(t *testing.T) {
	for name, column := range map[string]map[string]any{
		"court rate":        {"base_rate_cents": 3500},
		"standard hours":    {"base_hours": 3},
		"shuttles per hour": {"shuttles_per_hour": 4},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			admin := makeAdmin(t)
			player := makePlayer(t)
			session := pastSession(t, h, admin)

			body := map[string]any{
				"lines": []map[string]any{{"user_id": player.ID.String(), "in_base": true}},
			}
			fingerprint := h.previewFingerprint(admin, session.ID, body)

			if err := database.DB.Model(&models.Club{}).Where("1 = 1").Updates(column).Error; err != nil {
				t.Fatal(err)
			}

			before := readMoneyState(t)
			body["expected_preview"] = fingerprint
			expectPreviewChanged(t, h.as(admin).post(settlePath(session.ID), body))
			expectMoneyUnchanged(t, before)
		})
	}
}

// A rate the request does name is bound too: reviewing at one rate and
// confirming at another is the same mistake made by the client.
func TestSettleRefusesWhenTheRequestDiffersFromThePreview(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	other := makePlayer(t)
	session := pastSession(t, h, admin)

	line := func(u *models.User) map[string]any {
		return map[string]any{"user_id": u.ID.String(), "in_base": true}
	}
	reviewed := map[string]any{"lines": []map[string]any{line(player)}}
	fingerprint := h.previewFingerprint(admin, session.ID, reviewed)
	before := readMoneyState(t)

	for name, body := range map[string]map[string]any{
		"someone added":    {"lines": []map[string]any{line(player), line(other)}},
		"someone swapped":  {"lines": []map[string]any{line(other)}},
		"a guest added":    {"lines": []map[string]any{line(player), {"user_id": player.ID.String(), "guest_name": "Sam", "in_base": true}}},
		"someone comped":   {"lines": []map[string]any{{"user_id": player.ID.String(), "in_base": true, "comped": true}}},
		"an extra hour":    {"extra_hours": 1, "lines": []map[string]any{{"user_id": player.ID.String(), "in_base": true, "in_extra": true}}},
		"a different rate": {"base_rate_cents": 3500, "lines": []map[string]any{line(player)}},
	} {
		t.Run(name, func(t *testing.T) {
			h.t = t
			body["expected_preview"] = fingerprint
			h.as(admin).post(settlePath(session.ID), body).expect(http.StatusConflict)
			expectMoneyUnchanged(t, before)
		})
	}
	h.t = t
}

// Guests are settled on this form and nowhere else, so the review rule has to
// hold for a line that charges a host for someone who is not a member.
func TestSettleWithGuestsRequiresTheReviewedPreview(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	host := makePlayer(t)
	session := pastSession(t, h, admin)

	body := map[string]any{
		"lines": []map[string]any{
			{"user_id": host.ID.String(), "in_base": true},
			{"user_id": host.ID.String(), "guest_name": "Sam", "in_base": true},
		},
	}
	before := readMoneyState(t)
	h.as(admin).post(settlePath(session.ID), body).expect(http.StatusBadRequest)
	expectMoneyUnchanged(t, before)

	fingerprint := h.previewFingerprint(admin, session.ID, body)
	h.as(admin).post("/api/admin/transactions/shuttle-purchase", map[string]any{
		"units": 12, "amount_cents": 20000,
	}).expect(http.StatusCreated)

	before = readMoneyState(t)
	body["expected_preview"] = fingerprint
	expectPreviewChanged(t, h.as(admin).post(settlePath(session.ID), body))
	expectMoneyUnchanged(t, before)

	// Reviewed again at the new price, it settles, and the host carries both heads.
	var fresh struct {
		Fingerprint string `json:"fingerprint"`
		Totals      struct {
			ChargedCents int64 `json:"charged_cents"`
		} `json:"totals"`
	}
	delete(body, "expected_preview")
	h.as(admin).post(previewPath(session.ID), body).expect(http.StatusOK).decode(&fresh)
	body["expected_preview"] = fresh.Fingerprint
	h.as(admin).post(settlePath(session.ID), body).expect(http.StatusCreated)

	var balance struct {
		BalanceCents int64 `json:"balance_cents"`
	}
	h.as(host).get("/api/accounts/me").expect(http.StatusOK).decode(&balance)
	if balance.BalanceCents != -fresh.Totals.ChargedCents {
		t.Errorf("host balance = %d, want the reviewed %d", balance.BalanceCents, -fresh.Totals.ChargedCents)
	}
}

func TestSettleWithTheReviewedFingerprintSettlesExactlyOnce(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := pastSession(t, h, admin)

	body := map[string]any{
		"lines": []map[string]any{{"user_id": player.ID.String(), "in_base": true}},
	}
	var reviewed struct {
		Fingerprint string `json:"fingerprint"`
		Totals      struct {
			ChargedCents int64 `json:"charged_cents"`
		} `json:"totals"`
	}
	h.as(admin).post(previewPath(session.ID), body).expect(http.StatusOK).decode(&reviewed)
	body["expected_preview"] = reviewed.Fingerprint

	var settled struct {
		Totals struct {
			ChargedCents int64 `json:"charged_cents"`
		} `json:"totals"`
	}
	h.as(admin).post(settlePath(session.ID), body).expect(http.StatusCreated).decode(&settled)
	if settled.Totals.ChargedCents != reviewed.Totals.ChargedCents {
		t.Errorf("posted %d, reviewed %d", settled.Totals.ChargedCents, reviewed.Totals.ChargedCents)
	}

	// The same request again is refused as already settled, and posts nothing.
	after := readMoneyState(t)
	conflict := h.as(admin).post(settlePath(session.ID), body).expect(http.StatusConflict).ledgerError()
	if conflict.Code != "session_already_settled" {
		t.Errorf("code = %q, want session_already_settled", conflict.Code)
	}
	expectMoneyUnchanged(t, after)

	if after.Settlements != 1 || after.ChargeLines != 1 {
		t.Errorf("got %d settlements and %d charge lines, want 1 and 1", after.Settlements, after.ChargeLines)
	}
	var balance struct {
		BalanceCents int64 `json:"balance_cents"`
	}
	h.as(player).get("/api/accounts/me").expect(http.StatusOK).decode(&balance)
	if balance.BalanceCents != -reviewed.Totals.ChargedCents {
		t.Errorf("player balance = %d, want %d", balance.BalanceCents, -reviewed.Totals.ChargedCents)
	}
}
