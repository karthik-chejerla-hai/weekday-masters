package assistant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGroqTranscribesRecordedFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected request")
		}
		if err := r.ParseMultipartForm(1024); err != nil {
			t.Error(err)
		}
		if r.FormValue("model") != "whisper-large-v3-turbo" || r.FormValue("response_format") != "json" {
			t.Error("wrong transcription parameters")
		}
		_, file, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		if file.Filename != "recording.mp4" {
			t.Errorf("filename = %s", file.Filename)
		}
		w.Write([]byte(`{"text":"We used eight shuttles."}`))
	}))
	defer server.Close()
	p := NewGroq("test-key", "", "")
	p.baseURL = server.URL
	got, err := p.Transcribe(context.Background(), Audio{Data: []byte("audio"), MIME: "audio/mp4"})
	if err != nil || got != "We used eight shuttles." {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestGroqMapsToolsAndToolResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["parallel_tool_calls"] != false || body["include_reasoning"] != false {
			t.Error("unsupported model options")
		}
		messages := body["messages"].([]any)
		last := messages[len(messages)-1].(map[string]any)
		if last["tool_call_id"] != "call-1" || last["role"] != "tool" {
			t.Error("missing tool result ID")
		}
		w.Write([]byte(`{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call-2","type":"function","function":{"name":"get_club_position","arguments":"{}"}}]}}]}`))
	}))
	defer server.Close()
	p := NewGroq("test-key", "", "")
	p.baseURL = server.URL
	step, err := p.Next(context.Background(), []Message{{Role: "tool", Content: "{}", ToolCallID: "call-1"}}, []Tool{{Name: "get_club_position", Parameters: map[string]any{"type": "object"}}})
	if err != nil || len(step.Calls) != 1 || step.Calls[0].Name != "get_club_position" {
		t.Fatalf("step=%+v err=%v", step, err)
	}
}

func TestGroqFailuresAreSafe(t *testing.T) {
	for _, status := range []int{401, 429, 500, 200} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "12")
				w.WriteHeader(status)
				w.Write([]byte(`private provider detail`))
			}))
			defer server.Close()
			p := NewGroq("secret-key", "", "")
			p.baseURL = server.URL
			_, err := p.Next(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil)
			if err == nil || strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe error: %v", err)
			}
			if status == 429 {
				e, ok := err.(*Error)
				if !ok || e.RetryAfter != 12 || e.Status != 429 {
					t.Fatalf("quota: %#v", err)
				}
			}
		})
	}
}

func TestGroqMissingKeyAndCancellation(t *testing.T) {
	p := NewGroq("", "", "")
	if _, err := p.Next(context.Background(), nil, nil); err == nil {
		t.Fatal("missing key accepted")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	p = NewGroq("test", "", "")
	p.baseURL = server.URL
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Next(ctx, nil, nil); err == nil {
		t.Fatal("cancelled call succeeded")
	}
}

func TestGroqOtherModelOmitsReasoningOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if _, ok := body["include_reasoning"]; ok {
			t.Error("GPT-OSS options sent to another model")
		}
		if _, ok := body["reasoning_effort"]; ok {
			t.Error("GPT-OSS options sent to another model")
		}
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Hello"}}]}`))
	}))
	defer server.Close()
	p := NewGroq("test", "another-tool-model", "")
	p.baseURL = server.URL
	step, err := p.Next(context.Background(), []Message{{Role: "user", Content: "Hello"}}, nil)
	if err != nil || step.Text != "Hello" {
		t.Fatalf("%+v %v", step, err)
	}
}
