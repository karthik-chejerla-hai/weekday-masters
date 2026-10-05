// Package assistant defines the provider boundary. Business actions stay in services.
package assistant

import (
	"context"
	"encoding/json"
)

type Audio struct {
	Data []byte
	MIME string
}
type Transcriber interface {
	Transcribe(context.Context, Audio) (string, error)
}
type Planner interface {
	Next(context.Context, []Message, []Tool) (Step, error)
}
type Message struct {
	Role       string
	Content    string
	Calls      []ToolCall
	ToolCallID string
}
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
}
type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}
type Step struct {
	Text  string
	Calls []ToolCall
}

// Error contains only messages suitable for the caller, never provider responses.
type Error struct {
	Code       string
	Message    string
	Status     int
	RetryAfter int
}

func (e *Error) Error() string { return e.Message }
func Unavailable() *Error {
	return &Error{Code: "assistant_unavailable", Message: "The assistant is not configured. You can use the form.", Status: 503}
}
