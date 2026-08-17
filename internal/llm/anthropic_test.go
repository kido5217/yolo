package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func anthropicChecks(t *testing.T) func(*http.Request) {
	return func(r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("anthropic-version = %q", r.Header.Get("anthropic-version"))
		}
	}
}

func TestAnthropicBasicStream(t *testing.T) {
	srv := sseServer(t, "anthropic", "stream_basic.txt", anthropicChecks(t))
	defer srv.Close()
	parts := collect(t, stream(t, NewAnthropic(srv.Client()), Request{
		Model: "claude", APIKey: "test-key", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	}))
	if len(parts) != 3 { // text, text, finish
		t.Fatalf("parts = %+v", parts)
	}
	if parts[0].Kind != "text" || parts[0].Text != "Hey" || parts[1].Text != "! I am Claude." {
		t.Fatalf("deltas wrong: %+v", parts[:2])
	}
	if parts[2].Finish != "stop" {
		t.Fatalf("finish = %q", parts[2].Finish)
	}
	if parts[2].Usage == nil || parts[2].Usage.Input != 7 || parts[2].Usage.Output != 5 {
		t.Fatalf("usage = %+v", parts[2].Usage)
	}
}

func TestAnthropicThinkingAndToolUse(t *testing.T) {
	srv := sseServer(t, "anthropic", "stream_thinking_tool.txt", anthropicChecks(t))
	defer srv.Close()
	parts := collect(t, stream(t, NewAnthropic(srv.Client()), Request{
		Model: "claude", APIKey: "test-key", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "run it"}},
	}))
	var sawReasoning bool
	var tool Part
	var finish string
	for _, p := range parts {
		switch p.Kind {
		case "reasoning":
			sawReasoning = p.Text == "Let me check."
		case "tool":
			tool = p
		}
		if p.Finish != "" {
			finish = p.Finish
		}
	}
	if !sawReasoning || finish != "tool_calls" {
		t.Fatalf("reasoning=%v finish=%q parts=%+v", sawReasoning, finish, parts)
	}
	if tool.CallID != "toolu_1" || tool.Name != "bash" {
		t.Fatalf("tool = %+v", tool)
	}
	var args map[string]string
	if err := json.Unmarshal(tool.Args, &args); err != nil || args["command"] != "ls -la" {
		t.Fatalf("tool args = %s err=%v", tool.Args, err)
	}
}

func TestAnthropicMidStreamError(t *testing.T) {
	srv := sseServer(t, "anthropic", "midstream_error.txt", anthropicChecks(t))
	defer srv.Close()
	s := stream(t, NewAnthropic(srv.Client()), Request{
		Model: "claude", APIKey: "test-key", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "x"}},
	})
	first, _ := s.Next(ctx0(t))
	if first.Text != "oops" {
		t.Fatalf("first = %+v", first)
	}
	var final Part
	for {
		p, err := s.Next(ctx0(t))
		if p.Finish == "error" {
			final = p
			break
		}
		if err != nil {
			break
		}
	}
	if final.Finish != "error" || final.Err == nil {
		t.Fatalf("final = %+v", final)
	}
}

func TestAnthropicRequestShape(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "k" {
			t.Errorf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("anthropic-version = %q", r.Header.Get("anthropic-version"))
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer srv.Close()
	stream(t, NewAnthropic(srv.Client()), Request{
		Model: "claude", APIKey: "k", BaseURL: srv.URL,
		Messages:  []Message{{Role: RoleSystem, Content: "be nice"}, {Role: RoleUser, Content: "hi"}},
		Tools:     []ToolDef{{Name: "read", Description: "d", Parameters: json.RawMessage(`{"type":"object"}`)}},
		MaxTokens: 100,
	})
	if got["stream"] != true {
		t.Fatalf("stream = %v", got["stream"])
	}
	if got["max_tokens"] != float64(100) {
		t.Fatalf("max_tokens = %v", got["max_tokens"])
	}
	if got["system"] != "be nice" {
		t.Fatalf("system = %v", got["system"])
	}
	tools, _ := got["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %v", got["tools"])
	}
	tm := tools[0].(map[string]any)
	if tm["name"] != "read" || tm["description"] != "d" {
		t.Fatalf("tm = %v", tm)
	}
	if _, ok := tm["input_schema"].(map[string]any); !ok {
		t.Fatalf("input_schema missing: %v", tm)
	}
}
