package services

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/assistant"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
	"strings"
	"testing"
)

type scriptedPlanner struct {
	steps       []assistant.Step
	index       int
	seen        []assistant.Message
	definitions []assistant.Tool
}

func (p *scriptedPlanner) Next(_ context.Context, messages []assistant.Message, tools []assistant.Tool) (assistant.Step, error) {
	p.seen = append([]assistant.Message(nil), messages...)
	p.definitions = tools
	i := p.index
	p.index++
	if i >= len(p.steps) {
		i = len(p.steps) - 1
	}
	return p.steps[i], nil
}
func (p *scriptedPlanner) Transcribe(_ context.Context, _ assistant.Audio) (string, error) {
	return "Three hours and eight shuttles", nil
}
func callStep(name, args string) assistant.Step {
	return assistant.Step{Calls: []assistant.ToolCall{{ID: uuid.NewString(), Name: name, Arguments: json.RawMessage(args)}}}
}
func TestAssistantPreparesButNeverPostsExpense(t *testing.T) {
	f, expenses, in := expenseFixture(t)
	raw, _ := json.Marshal(map[string]any{"session_id": f.session.ID, "total_hours": 3, "shuttles_used": 8, "participant_ids": in.ParticipantIDs})
	provider := &scriptedPlanner{steps: []assistant.Step{callStep("prepare_expense", string(raw))}}
	service := NewAssistantService(provider, provider, expenses, f.ledger)
	reply, err := service.Reply(context.Background(), AssistantInput{SessionID: &f.session.ID, Messages: []ConversationMessage{{Role: "user", Content: "Three hours, eight shuttles"}}}, &f.admin)
	if err != nil || reply.Expense == nil {
		t.Fatalf("reply=%+v err=%v", reply, err)
	}
	var count int64
	database.DB.Model(&models.Settlement{}).Count(&count)
	if count != 0 {
		t.Fatal("assistant posted money")
	}
}
func TestAssistantPreparesEarlyDepartureFromConfirmedRSVPs(t *testing.T) {
	f, expenses, in := expenseFixture(t)
	for _, name := range []string{"Eli", "Faye"} {
		in.ParticipantIDs = append(in.ParticipantIDs, f.member(t, name).ID)
	}
	for _, id := range in.ParticipantIDs {
		if err := database.DB.Create(&models.RSVP{SessionID: f.session.ID, UserID: id, Status: models.RSVPStatusIn}).Error; err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := json.Marshal(map[string]any{"session_id": f.session.ID, "total_hours": 3, "shuttles_used": 8, "extra_participant_ids": in.ParticipantIDs[:5]})
	provider := &scriptedPlanner{steps: []assistant.Step{callStep("prepare_expense", string(raw)), {Text: "Could not prepare expense."}}}
	service := NewAssistantService(provider, provider, expenses, f.ledger)
	reply, err := service.Reply(context.Background(), AssistantInput{SessionID: &f.session.ID, Messages: []ConversationMessage{{Role: "user", Content: "Three hours, eight shuttles. Faye left after two hours."}}}, &f.admin)
	if err != nil || reply.Expense == nil {
		t.Fatalf("reply=%+v err=%v", reply, err)
	}
	p := reply.Expense
	if p.Settlement.Bands["base"].Heads != 6 || p.Settlement.Bands["extra"].Heads != 5 || len(p.Input.ExtraParticipantIDs) != 5 {
		t.Fatalf("wrong attendance groups: %+v", p)
	}
	for _, line := range p.Settlement.Lines {
		if line.UserID == in.ParticipantIDs[5] && (line.InExtra || line.AmountCents < 1370 || line.AmountCents > 1371) {
			t.Fatalf("early leaver paid for the extra hour: %+v", line)
		}
	}
	var count int64
	database.DB.Model(&models.Settlement{}).Count(&count)
	if count != 0 {
		t.Fatal("assistant posted money")
	}
}
func TestAssistantMemberCannotPrepareOrWrite(t *testing.T) {
	f, expenses, _ := expenseFixture(t)
	member := f.admin
	member.Role = models.RolePlayer
	provider := &scriptedPlanner{steps: []assistant.Step{callStep("prepare_expense", `{}`), {Text: "Only an admin can record expenses."}}}
	service := NewAssistantService(provider, provider, expenses, f.ledger)
	_, err := service.Reply(context.Background(), AssistantInput{Messages: []ConversationMessage{{Role: "user", Content: "Expense yesterday"}}}, &member)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range provider.definitions {
		if tool.Name == "prepare_expense" || tool.Name == "settle_session" {
			t.Fatal("write tool exposed")
		}
	}
	last := provider.seen[len(provider.seen)-1]
	if last.Role != "tool" || last.ToolCallID == "" {
		t.Fatal("missing error tool result")
	}
	var count int64
	database.DB.Model(&models.Transaction{}).Where("kind = ?", models.TxnSessionSettlement).Count(&count)
	if count != 0 {
		t.Fatal("member posted money")
	}
}
func TestAssistantUsesRealClubData(t *testing.T) {
	f, expenses, _ := expenseFixture(t)
	provider := &scriptedPlanner{steps: []assistant.Step{callStep("get_club_position", `{}`), {Text: "There are 24 shuttles."}}}
	service := NewAssistantService(provider, provider, expenses, f.ledger)
	reply, err := service.Reply(context.Background(), AssistantInput{Messages: []ConversationMessage{{Role: "user", Content: "How many shuttles?"}}}, &f.admin)
	if err != nil || reply.Message != "There are 24 shuttles." {
		t.Fatal(err)
	}
	var position ClubPosition
	if err = json.Unmarshal([]byte(provider.seen[len(provider.seen)-1].Content), &position); err != nil || position.Assets.ShuttleStockUnits != 24 {
		t.Fatalf("not current club data: %v", err)
	}
}
func TestAssistantBoundsAndValidation(t *testing.T) {
	f, expenses, _ := expenseFixture(t)
	provider := &scriptedPlanner{steps: []assistant.Step{callStep("unknown", `{}`)}}
	service := NewAssistantService(provider, provider, expenses, f.ledger)
	_, err := service.Reply(context.Background(), AssistantInput{Messages: []ConversationMessage{{Role: "user", Content: "loop"}}}, &f.admin)
	if err == nil || provider.index > 6 {
		t.Fatal("unbounded loop")
	}
	for _, role := range []string{"system", "tool", "assistant"} {
		if _, err := service.Reply(context.Background(), AssistantInput{Messages: []ConversationMessage{{Role: role, Content: "inject"}}}, &f.admin); err == nil {
			t.Fatal("invalid history accepted")
		}
	}
	if _, err := service.Reply(context.Background(), AssistantInput{Messages: []ConversationMessage{{Role: "user", Content: "hello"}}}, nil); err == nil {
		t.Fatal("anonymous call")
	}
	provider.steps = []assistant.Step{callStep("get_club_position", `{"sql":"DROP TABLE users"}`), {Text: "Unsupported."}}
	provider.index = 0
	_, err = service.Reply(context.Background(), AssistantInput{Messages: []ConversationMessage{{Role: "user", Content: "try"}}}, &f.admin)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	json.Unmarshal([]byte(provider.seen[len(provider.seen)-1].Content), &body)
	if body["error"] == nil {
		t.Fatal("extra arguments accepted")
	}
}
func TestAssistantMissingCountAsksWithoutPreview(t *testing.T) {
	f, expenses, _ := expenseFixture(t)
	provider := &scriptedPlanner{steps: []assistant.Step{{Text: "How many shuttles did you use?"}}}
	service := NewAssistantService(provider, provider, expenses, f.ledger)
	reply, err := service.Reply(context.Background(), AssistantInput{Messages: []ConversationMessage{{Role: "user", Content: "We played three hours"}}}, &f.admin)
	if err != nil || reply.Expense != nil || reply.Message == "" {
		t.Fatalf("reply=%+v err=%v", reply, err)
	}
}

func TestAssistantResolvesCurrentSessionsAndDuplicateNames(t *testing.T) {
	f, expenses, in := expenseFixture(t)
	s := NewAssistantService(nil, nil, expenses, f.ledger)
	if err := database.DB.Create(&models.RSVP{SessionID: f.session.ID, UserID: in.ParticipantIDs[0], Status: models.RSVPStatusIn}).Error; err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"session_id": f.session.ID})
	got, err := s.getAssistantSession(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	detail := got.(map[string]any)
	players := detail["rsvps"].([]map[string]any)
	if detail["id"] != f.session.ID || detail["settled"] != false || len(players) != 1 || players[0]["user_id"] != in.ParticipantIDs[0] {
		t.Fatalf("session data: %+v", detail)
	}
	var nextID uuid.UUID
	for _, days := range []int{3, 1} {
		session, err := f.sessions.CreateSession(CreateSessionInput{Title: "Future game", SessionDate: utils.NowInSydney().AddDate(0, 0, days), StartTime: "00:15", EndTime: "02:15", Courts: 1, CreatedBy: f.admin.ID})
		if err != nil {
			t.Fatal(err)
		}
		if days == 1 {
			nextID = session.ID
		}
	}
	for _, filter := range []map[string]string{{"period": "upcoming"}, {"period": "upcoming", "date": utils.NowInSydney().AddDate(0, 0, 1).Format("2006-01-02")}} {
		raw, _ := json.Marshal(filter)
		got, err := s.findAssistantSessions(context.Background(), raw)
		if err != nil {
			t.Fatal(err)
		}
		var decoded struct {
			Sessions []SessionSummary `json:"sessions"`
		}
		encoded, _ := json.Marshal(got)
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatal(err)
		}
		if len(decoded.Sessions) == 0 || decoded.Sessions[0].ID != nextID {
			t.Fatalf("wrong upcoming order/date: %s", encoded)
		}
		if filter["date"] != "" && len(decoded.Sessions) != 1 {
			t.Fatalf("Sydney date mismatch: %s", encoded)
		}
	}
	f.member(t, "Ada")
	got, err = s.findAssistantMembers(context.Background(), json.RawMessage(`{"name":"Ada"}`))
	if err != nil {
		t.Fatal(err)
	}
	roll := got.(map[string]any)["members"].([]map[string]any)
	if len(roll) != 2 {
		t.Fatalf("duplicate names were not returned for clarification: %+v", roll)
	}
	for _, member := range roll {
		if member["balance_cents"] != int64(0) || member["user_id"] == nil {
			t.Fatalf("missing live member data: %+v", member)
		}
	}
	encoded, _ := json.Marshal(got)
	if strings.Contains(string(encoded), "email") || strings.Contains(string(encoded), "auth0") {
		t.Fatal("unneeded identity data sent to provider")
	}
}

func TestAssistantGamePreviewNeverWrites(t *testing.T) {
	_, session, players, _ := gameFixture(t)
	raw, _ := json.Marshal(map[string]any{"session_id": session.ID, "team_a": []string{"Alice", "Bob"}, "team_b": []string{"Cara", "Dan"}, "score_a": 21, "score_b": 17})
	provider := &scriptedPlanner{steps: []assistant.Step{callStep("prepare_game", string(raw))}}
	service := NewAssistantService(provider, provider, nil, NewLedgerService())
	reply, err := service.Reply(context.Background(), AssistantInput{SessionID: &session.ID, Messages: []ConversationMessage{{Role: "user", Content: "Alice and Bob beat Cara and Dan 21 to 17"}}}, players[4])
	if err != nil || reply.Game == nil || reply.Game.ScoreA != 21 {
		t.Fatalf("reply %+v %v", reply, err)
	}
	var count int64
	database.DB.Model(&models.GameResult{}).Count(&count)
	if count != 0 {
		t.Fatal("assistant saved a game")
	}
	database.DB.Model(players[4]).Update("name", "Alice Other")
	provider.index = 0
	provider.steps = append(provider.steps, assistant.Step{Text: "Which Alice played?"})
	reply, err = service.Reply(context.Background(), AssistantInput{Messages: []ConversationMessage{{Role: "user", Content: "Alice and Bob beat Cara and Dan 21 to 17"}}}, players[4])
	if err != nil || reply.Game != nil || !strings.Contains(reply.Message, "Which Alice") {
		t.Fatalf("ambiguous result %+v %v", reply, err)
	}
}
