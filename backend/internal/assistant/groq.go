package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const MaxAudioBytes = 10 << 20

// Groq implements speech and planning independently. Its wire types stay here.
type Groq struct {
	key, model, speechModel, baseURL string
	client                           *http.Client
}

func NewGroq(key, model, speechModel string) *Groq {
	if model == "" {
		model = "openai/gpt-oss-120b"
	}
	if speechModel == "" {
		speechModel = "whisper-large-v3-turbo"
	}
	return &Groq{key: key, model: model, speechModel: speechModel, baseURL: "https://api.groq.com/openai/v1", client: &http.Client{Timeout: 35 * time.Second}}
}

func AudioExtension(contentType string) (string, bool) {
	kind, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", false
	}
	ext, ok := map[string]string{"audio/webm": "webm", "video/webm": "webm", "audio/mp4": "mp4", "video/mp4": "mp4", "audio/m4a": "m4a", "audio/x-m4a": "m4a", "audio/ogg": "ogg", "audio/wav": "wav", "audio/x-wav": "wav", "audio/mpeg": "mp3", "audio/flac": "flac"}[kind]
	return ext, ok
}

func (g *Groq) Transcribe(ctx context.Context, audio Audio) (string, error) {
	ext, ok := AudioExtension(audio.MIME)
	if !ok || len(audio.Data) == 0 || len(audio.Data) > MaxAudioBytes {
		return "", &Error{Code: "invalid_audio", Message: "Use a supported recording smaller than 10 MB.", Status: 400}
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", "recording."+ext)
	if err != nil {
		return "", providerFailure()
	}
	if _, err = part.Write(audio.Data); err != nil {
		return "", providerFailure()
	}
	for k, v := range map[string]string{"model": g.speechModel, "response_format": "json", "language": "en"} {
		if err = form.WriteField(k, v); err != nil {
			return "", providerFailure()
		}
	}
	if err = form.Close(); err != nil {
		return "", providerFailure()
	}
	var result struct {
		Text string `json:"text"`
	}
	if err = g.request(ctx, "/audio/transcriptions", form.FormDataContentType(), &body, &result); err != nil {
		return "", err
	}
	result.Text = strings.TrimSpace(result.Text)
	if result.Text == "" || len(result.Text) > 4000 {
		return "", &Error{Code: "no_speech", Message: "No clear speech was recognised. Try again or type your request.", Status: 422}
	}
	return result.Text, nil
}

type groqCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}
type groqMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []groqCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

func (g *Groq) Next(ctx context.Context, messages []Message, tools []Tool) (Step, error) {
	wire := make([]groqMessage, 0, len(messages))
	for _, m := range messages {
		next := groqMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
		for _, c := range m.Calls {
			call := groqCall{ID: c.ID, Type: "function"}
			call.Function.Name = c.Name
			call.Function.Arguments = string(c.Arguments)
			next.ToolCalls = append(next.ToolCalls, call)
		}
		wire = append(wire, next)
	}
	definitions := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		definitions = append(definitions, map[string]any{"type": "function", "function": map[string]any{"name": t.Name, "description": t.Description, "parameters": t.Parameters}})
	}
	payload := map[string]any{"model": g.model, "messages": wire, "tools": definitions, "tool_choice": "auto", "parallel_tool_calls": false, "max_completion_tokens": 1200}
	if strings.HasPrefix(g.model, "openai/gpt-oss-") {
		payload["include_reasoning"] = false
		payload["reasoning_effort"] = "low"
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Step{}, providerFailure()
	}
	var result struct {
		Choices []struct {
			Message groqMessage `json:"message"`
		} `json:"choices"`
	}
	if err = g.request(ctx, "/chat/completions", "application/json", bytes.NewReader(body), &result); err != nil {
		return Step{}, err
	}
	if len(result.Choices) != 1 {
		return Step{}, providerFailure()
	}
	message := result.Choices[0].Message
	step := Step{Text: strings.TrimSpace(message.Content)}
	if len(message.ToolCalls) > 4 {
		return Step{}, providerFailure()
	}
	for _, c := range message.ToolCalls {
		if c.ID == "" || c.Type != "function" || !json.Valid([]byte(c.Function.Arguments)) {
			return Step{}, providerFailure()
		}
		step.Calls = append(step.Calls, ToolCall{ID: c.ID, Name: c.Function.Name, Arguments: json.RawMessage(c.Function.Arguments)})
	}
	if step.Text == "" && len(step.Calls) == 0 {
		return Step{}, providerFailure()
	}
	return step, nil
}

func providerFailure() *Error {
	return &Error{Code: "assistant_provider_error", Message: "The assistant could not respond. Try again or use the form.", Status: 502}
}

func (g *Groq) request(ctx context.Context, path, contentType string, body io.Reader, result any) error {
	if strings.TrimSpace(g.key) == "" {
		return Unavailable()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+path, body)
	if err != nil {
		return providerFailure()
	}
	req.Header.Set("Authorization", "Bearer "+g.key)
	req.Header.Set("Content-Type", contentType)
	res, err := g.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return &Error{Code: "assistant_timeout", Message: "The assistant request ended. Try again or use the form.", Status: 504}
		}
		return providerFailure()
	}
	defer res.Body.Close()
	if res.StatusCode == 429 {
		retry, _ := strconv.Atoi(res.Header.Get("Retry-After"))
		if retry < 1 || retry > 3600 {
			retry = 60
		}
		return &Error{Code: "assistant_rate_limited", Message: "The assistant has reached its usage limit. Try again later or use the form.", Status: 429, RetryAfter: retry}
	}
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return &Error{Code: "assistant_configuration", Message: "The assistant key needs attention. You can use the expense form.", Status: 503}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return providerFailure()
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20+1))
	if err != nil || len(raw) > 1<<20 || json.Unmarshal(raw, result) != nil {
		return providerFailure()
	}
	return nil
}
