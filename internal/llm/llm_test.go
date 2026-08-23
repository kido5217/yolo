package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	drainCtx := ctx0(t) // one timeout bounds the whole drain
	for {
		p, err := s.Next(drainCtx)
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

// TestOpenAIClosesStreamAtDone: a server that sends [DONE] but keeps the
// connection open (no body EOF) must not stall the stream — consumers drain
// to io.EOF (the engine's round loop), so the channel has to close at
// [DONE], not at EOF (anReadSSE parity, anthropic.go:263).
func TestOpenAIClosesStreamAtDone(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	defer close(release)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fl, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hi\"}}]}\n\n"))
		fl.Flush()
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		fl.Flush()
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		fl.Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	s := stream(t, NewOpenAI(srv.Client()), Request{
		Model: "m", APIKey: "test-key", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	var parts []Part
	drainCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		p, err := s.Next(drainCtx)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				t.Fatalf("drain ended with %v, want io.EOF (stream stuck past [DONE])", err)
			}
			break
		}
		parts = append(parts, p)
	}
	if len(parts) != 2 {
		t.Fatalf("parts = %+v, want text+finish", parts)
	}
	if parts[0].Kind != "text" || parts[0].Text != "hi" {
		t.Fatalf("first = %+v", parts[0])
	}
	if parts[1].Finish != "stop" {
		t.Fatalf("finish = %+v", parts[1])
	}
}

func TestOpenAIUpstream429IsTransient(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	if got["max_tokens"] != float64(100) {
		t.Fatalf("max_tokens = %v", got["max_tokens"])
	}
	msgs, _ := got["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages = %v", got["messages"])
	}
	m0 := msgs[0].(map[string]any)
	if m0["role"] != "system" || m0["content"] != "sys" {
		t.Fatalf("msg0 = %v", m0)
	}
	m1 := msgs[1].(map[string]any)
	if m1["role"] != "user" || m1["content"] != "hi" {
		t.Fatalf("msg1 = %v", m1)
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

// TestOpenAIUpstream400DrainsBody pins the ④ fix: the non-2xx body is read
// (capped) BEFORE close, so the provider error.message surfaces in the error.
func TestOpenAIUpstream400DrainsBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"message":"prompt is too long: 120000 tokens > 100000 maximum"}}`))
	}))
	defer srv.Close()
	_, err := NewOpenAI(srv.Client()).Stream(ctx0(t), Request{
		Model: "m", APIKey: "k", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "x"}},
	})
	if err == nil || IsTransient(err) {
		t.Fatalf("err = %v, want non-transient API error", err)
	}
	if !strings.Contains(err.Error(), "prompt is too long: 120000 tokens > 100000 maximum") {
		t.Fatalf("decoded message missing: %v", err)
	}
	if !strings.Contains(err.Error(), "upstream error (http 400):") {
		t.Fatalf("status framing missing: %v", err)
	}
	var api *APIError
	if !errors.As(err, &api) {
		t.Fatalf("no *APIError in chain: %v", err)
	}
	if api.Status != 400 || api.Message != "prompt is too long: 120000 tokens > 100000 maximum" {
		t.Fatalf("APIError = %+v", api)
	}
}

// TestOpenAIUpstream429DecodesMessage pins that the 429 envelope message is
// decoded (it used to be lost to the pre-close) while staying transient.
func TestOpenAIUpstream429DecodesMessage(t *testing.T) {
	t.Parallel()
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
	if !strings.Contains(err.Error(), "slow down") {
		t.Fatalf("decoded message missing: %v", err)
	}
}

// TestOpenAIUpstreamBodyCap pins the 64 KiB drain cap: an oversized body is
// truncated (no hang, no full-body error text) and the cap is visible on the
// APIError.
func TestOpenAIUpstreamBodyCap(t *testing.T) {
	t.Parallel()
	big := bytes.Repeat([]byte("x"), 200*1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(413)
		_, _ = w.Write(big)
	}))
	defer srv.Close()
	_, err := NewOpenAI(srv.Client()).Stream(ctx0(t), Request{
		Model: "m", APIKey: "k", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "x"}},
	})
	var api *APIError
	if err == nil || !errors.As(err, &api) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if len(api.Body) != 64*1024 {
		t.Fatalf("body cap = %d, want %d", len(api.Body), 64*1024)
	}
}

// TestAnthropicUpstream400DrainsBody pins the same ④ fix on the anthropic
// driver (shared upstreamError).
func TestAnthropicUpstream400DrainsBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"input is too long for requested model"}}`))
	}))
	defer srv.Close()
	_, err := NewAnthropic(srv.Client()).Stream(ctx0(t), Request{
		Model: "m", APIKey: "k", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "x"}},
	})
	if err == nil || IsTransient(err) {
		t.Fatalf("err = %v, want non-transient API error", err)
	}
	if !strings.Contains(err.Error(), "input is too long for requested model") {
		t.Fatalf("decoded message missing: %v", err)
	}
	var api *APIError
	if !errors.As(err, &api) || api.Status != 400 {
		t.Fatalf("APIError = %v", err)
	}
}
