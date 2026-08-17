package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func openAIChecks(t *testing.T) func(*http.Request) {
	return func(r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
	}
}

func TestOpenAIBasicStream(t *testing.T) {
	srv := sseServer(t, "openai", "stream_basic.txt", openAIChecks(t))
	defer srv.Close()
	parts := collect(t, stream(t, NewOpenAI(srv.Client()), Request{
		Model: "m", APIKey: "test-key", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	}))
	if len(parts) != 3 { // text, text, finish
		t.Fatalf("parts = %+v", parts)
	}
	if parts[0].Kind != "text" || parts[0].Text != "Hel" || parts[1].Text != "lo" {
		t.Fatalf("deltas wrong: %+v", parts[:2])
	}
	if parts[2].Finish != "stop" {
		t.Fatalf("finish = %q", parts[2].Finish)
	}
}

func TestOpenAIMidFrameSplits(t *testing.T) {
	srv := sseServerSplit(t, "openai", "stream_split_frames.txt", openAIChecks(t))
	defer srv.Close()
	parts := collect(t, stream(t, NewOpenAI(srv.Client()), Request{
		Model: "m", APIKey: "test-key", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	}))
	var text string
	for _, p := range parts {
		if p.Kind == "text" {
			text += p.Text
		}
	}
	if text != "ab" {
		t.Fatalf("reassembled text = %q", text)
	}
}

func TestOpenAIReasoningAndToolCalls(t *testing.T) {
	srv := sseServer(t, "openai", "stream_reasoning_tools.txt", openAIChecks(t))
	defer srv.Close()
	parts := collect(t, stream(t, NewOpenAI(srv.Client()), Request{
		Model: "m", APIKey: "test-key", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "use tools"}},
	}))
	var sawReasoning, sawText bool
	tools := map[string]Part{}
	var finish string
	for _, p := range parts {
		switch p.Kind {
		case "reasoning":
			sawReasoning = p.Text == "thinking..."
		case "text":
			sawText = sawText || p.Text == "answer "
		case "tool":
			tools[p.CallID] = p
		}
		if p.Finish != "" {
			finish = p.Finish
		}
	}
	if !sawReasoning || !sawText || finish != "tool_calls" {
		t.Fatalf("reasoning=%v text=%v finish=%q parts=%+v", sawReasoning, sawText, finish, parts)
	}
	tc, ok := tools["call_1"]
	if !ok || tc.Name != "read" {
		t.Fatalf("tool part = %+v (tools=%v)", tc, tools)
	}
	var args map[string]string
	if err := json.Unmarshal(tc.Args, &args); err != nil || args["filePath"] != "/x" {
		t.Fatalf("tool args = %s err=%v", tc.Args, err)
	}
}

func TestOpenAIUsageFinal(t *testing.T) {
	srv := sseServer(t, "openai", "stream_usage_only_final.txt", openAIChecks(t))
	defer srv.Close()
	parts := collect(t, stream(t, NewOpenAI(srv.Client()), Request{
		Model: "m", APIKey: "test-key", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "x"}},
	}))
	var u *Usage
	var finish string
	for _, p := range parts {
		if p.Usage != nil {
			u = p.Usage
		}
		if p.Finish != "" {
			finish = p.Finish
		}
	}
	if u == nil || u.Input != 1 || u.Output != 1 || u.Reasoning != 1 || u.CacheRead != 1 {
		t.Fatalf("usage = %+v", u)
	}
	if finish != "length" {
		t.Fatalf("finish = %q", finish)
	}
}

func TestOpenAIMidStreamError(t *testing.T) {
	srv := sseServer(t, "openai", "midstream_error.txt", openAIChecks(t))
	defer srv.Close()
	s := stream(t, NewOpenAI(srv.Client()), Request{
		Model: "m", APIKey: "test-key", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "x"}},
	})
	first, _ := s.Next(ctx0(t))
	if first.Text != "partial" {
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

func TestOpenAIUpstream429IsTransient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":{"message":"slow down"}}`))
	}))
	defer srv.Close()
	_, err := NewOpenAI(srv.Client()).Stream(ctx0(t), Request{
		Model: "m", APIKey: "k", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "x"}},
	})
	if err == nil || !IsTransient(err) {
		t.Fatalf("err = %v, want transient", err)
	}
}

func TestOpenAIRequestShape(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	stream(t, NewOpenAI(srv.Client()), Request{
		Model: "m", APIKey: "k", BaseURL: srv.URL,
		Messages:  []Message{{Role: RoleSystem, Content: "sys"}, {Role: RoleUser, Content: "hi"}},
		Tools:     []ToolDef{{Name: "read", Description: "d", Parameters: json.RawMessage(`{"type":"object"}`)}},
		MaxTokens: 100,
	})
	if got["stream"] != true {
		t.Fatalf("stream = %v", got["stream"])
	}
	tools, _ := got["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %v", got["tools"])
	}
	fn := tools[0].(map[string]any)["function"].(map[string]any)
	if fn["name"] != "read" {
		t.Fatalf("fn = %v", fn)
	}
}
