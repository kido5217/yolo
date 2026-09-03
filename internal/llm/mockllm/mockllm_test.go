package mockllm

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const cannedReply = "## Heading\n\nSome **bold** and `inline code` text.\n\n- one\n- two\n\n> a quote\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n[link](https://example.com)\n\n```js\nconst x = 1;\n```\n\n你好 world\n\nDone."

const toolCannedArgs = `{"command":"echo parity-ok"}`
const toolCannedReply = "The check printed parity-ok.\n"
const todoCannedArgs = `{"todos":[{"content":"first item","status":"in_progress"},{"content":"second item","status":"pending"}]}`
const todoCannedReply = "Todos updated.\n"

func textCanned() Canned {
	return Canned{Prompt: "say hi to the mock", Reply: cannedReply, ChunkSize: 6, Usage: Usage{Input: 12, Output: 40}}
}

func toolCanned() Canned {
	return Canned{
		Prompt:    "run the parity check",
		Tool:      &Tool{Name: "bash", Args: toolCannedArgs},
		ToolReply: toolCannedReply,
		ChunkSize: 6,
		Usage:     Usage{Input: 12, Output: 40},
	}
}

func todoCanned() Canned {
	return Canned{
		Prompt:    "add two todos",
		Tool:      &Tool{Name: "todowrite", Args: todoCannedArgs},
		ToolReply: todoCannedReply,
		ChunkSize: 6,
		Usage:     Usage{Input: 12, Output: 40},
	}
}

type testFrame struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// postStream runs one chat/completions request and returns the "data:"
// payloads in order (the last is [DONE]).
func postStream(t *testing.T, c Canned, body string) []string {
	t.Helper()
	srv := httptest.NewServer(Handler(c))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "data: ") {
			out = append(out, strings.TrimPrefix(line, "data: "))
		}
	}
	if len(out) == 0 || out[len(out)-1] != "[DONE]" {
		t.Fatalf("expected a [DONE] terminator, got %d frames: %q", len(out), out)
	}
	return out
}

func frames(t *testing.T, payloads []string) []testFrame {
	t.Helper()
	var out []testFrame
	for _, p := range payloads {
		if p == "[DONE]" {
			continue
		}
		var f testFrame
		if err := json.Unmarshal([]byte(p), &f); err != nil {
			t.Fatalf("frame %q: %v", p, err)
		}
		out = append(out, f)
	}
	return out
}

func pinMeta(t *testing.T, fs []testFrame) {
	t.Helper()
	for i, f := range fs {
		if f.ID != "chatcmpl-canned01" || f.Object != "chat.completion.chunk" ||
			f.Created != 1700000000 || f.Model != "canned" {
			t.Fatalf("frame %d meta = %+v (want the fixed id/object/created/model)", i, f)
		}
	}
}

// TestCannedStreamFrames pins the text-turn frame order (D1): role chunk →
// ≤6-rune content chunks (the re-join is the canned reply) → the finish
// chunk → the usage chunk → [DONE].
func TestCannedStreamFrames(t *testing.T) {
	p := postStream(t, textCanned(), `{"stream":true,"messages":[{"role":"user","content":"say hi to the mock"}]}`)
	fs := frames(t, p)
	if len(fs) < 4 {
		t.Fatalf("frame count = %d, want >= 4 (role + content + finish + usage)", len(fs))
	}
	pinMeta(t, fs)
	if got := fs[0].Choices[0].Delta; got.Role != "assistant" || got.Content != "" {
		t.Fatalf("first frame delta = %+v, want role=assistant content empty", got)
	}
	var joined string
	for _, f := range fs[1 : len(fs)-2] {
		if len(f.Choices) != 1 {
			t.Fatalf("content frame: %d choices", len(f.Choices))
		}
		d := f.Choices[0].Delta
		if d.Role != "" || len([]rune(d.Content)) == 0 || len([]rune(d.Content)) > 6 {
			t.Fatalf("bad content chunk: %+v", d)
		}
		joined += d.Content
	}
	if joined != cannedReply {
		t.Fatalf("re-joined content != canned reply:\n got %q\nwant %q", joined, cannedReply)
	}
	fin := fs[len(fs)-2]
	if fin.Choices[0].FinishReason == nil || *fin.Choices[0].FinishReason != "stop" {
		t.Fatalf("finish chunk = %+v, want finish_reason=stop", fin.Choices[0])
	}
	ug := fs[len(fs)-1]
	if len(ug.Choices) != 0 || ug.Usage == nil ||
		ug.Usage.PromptTokens != 12 || ug.Usage.CompletionTokens != 40 || ug.Usage.TotalTokens != 52 {
		t.Fatalf("usage chunk = %+v, want empty choices + 12/40/52", ug)
	}
}

// TestToolTurn pins the tool-call frame order + the post-result follow-up
// (D1): role → tc-id → tc-args → finish(tool_calls) → usage → [DONE],
// then the ToolReply text turn after the tool result is posted.
func TestToolTurn(t *testing.T) {
	p := postStream(t, toolCanned(), `{"stream":true,"messages":[{"role":"user","content":"run the parity check"}]}`)
	fs := frames(t, p)
	if len(fs) != 5 {
		t.Fatalf("tool frame count = %d, want 5 (role + tc-id + tc-args + finish + usage)", len(fs))
	}
	pinMeta(t, fs)
	if tc := fs[1].Choices[0].Delta.ToolCalls; len(tc) != 1 || tc[0].ID != "call_canned1" ||
		tc[0].Type != "function" || tc[0].Function.Name != "bash" || tc[0].Function.Arguments != "" {
		t.Fatalf("tc-id frame = %+v", tc)
	}
	if tc := fs[2].Choices[0].Delta.ToolCalls; len(tc) != 1 || tc[0].Function.Arguments != toolCannedArgs {
		t.Fatalf("tc-args frame = %+v", tc)
	}
	if fr := fs[3].Choices[0].FinishReason; fr == nil || *fr != "tool_calls" {
		t.Fatalf("tool finish = %+v, want tool_calls", fs[3].Choices[0])
	}
	reqBody, err := json.Marshal(map[string]any{
		"stream": true,
		"messages": []any{
			map[string]string{"role": "user", "content": "run the parity check"},
			map[string]any{
				"role":    "assistant",
				"content": "",
				"tool_calls": []any{map[string]any{
					"id":       "call_canned1",
					"function": map[string]string{"name": "bash", "arguments": toolCannedArgs},
				}},
			},
			map[string]string{"role": "tool", "tool_call_id": "call_canned1", "content": "parity-ok\n"},
		},
	})
	if err != nil {
		t.Fatalf("marshal follow-up body: %v", err)
	}
	p2 := postStream(t, toolCanned(), string(reqBody))
	fs2 := frames(t, p2)
	var joined string
	for _, f := range fs2[1 : len(fs2)-2] {
		joined += f.Choices[0].Delta.Content
	}
	if joined != toolCannedReply {
		t.Fatalf("follow-up re-join = %q, want %q", joined, toolCannedReply)
	}
}

// TestTodoTurn pins the todowrite call id + args (the D1 third turn).
func TestTodoTurn(t *testing.T) {
	p := postStream(t, todoCanned(), `{"stream":true,"messages":[{"role":"user","content":"add two todos"}]}`)
	fs := frames(t, p)
	if len(fs) != 5 {
		t.Fatalf("todo frame count = %d, want 5", len(fs))
	}
	if tc := fs[1].Choices[0].Delta.ToolCalls; len(tc) != 1 || tc[0].ID != "call_canned2" ||
		tc[0].Function.Name != "todowrite" {
		t.Fatalf("todo tc-id frame = %+v", tc)
	}
	if tc := fs[2].Choices[0].Delta.ToolCalls; len(tc) != 1 || tc[0].Function.Arguments != todoCannedArgs {
		t.Fatalf("todo tc-args frame = %+v", tc)
	}
}

// TestNonStream pins the non-stream completion body.
func TestNonStream(t *testing.T) {
	srv := httptest.NewServer(Handler(textCanned()))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json",
		strings.NewReader(`{"stream":false,"messages":[{"role":"user","content":"say hi to the mock"}]}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	var b struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if b.ID != "chatcmpl-canned01" || b.Object != "chat.completion" || b.Created != 1700000000 ||
		b.Model != "canned" || len(b.Choices) != 1 ||
		b.Choices[0].Message.Content != cannedReply || b.Choices[0].FinishReason != "stop" ||
		b.Usage.PromptTokens != 12 || b.Usage.TotalTokens != 52 {
		t.Fatalf("non-stream body = %+v", b)
	}
}

// TestModelsEndpoint pins the GET /models body (byte-identical).
func TestModelsEndpoint(t *testing.T) {
	srv := httptest.NewServer(Handler(textCanned()))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/v1/models")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(raw, modelsJSON) {
		t.Fatalf("models body = %s, want the pinned body", raw)
	}
}

// TestDeterminism pins byte-identical streams across two handler
// instances (the capture-determinism gate, D5's Go-side referent).
func TestDeterminism(t *testing.T) {
	body := `{"stream":true,"messages":[{"role":"user","content":"say hi to the mock"}]}`
	getBody := func() []byte {
		srv := httptest.NewServer(Handler(textCanned()))
		defer srv.Close()
		resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		return raw
	}
	if !bytes.Equal(getBody(), getBody()) {
		t.Fatal("two handler instances produced different streams")
	}
}

// TestCannedMatchesDefault pins the shared fixture against DefaultBook
// (D1, root principle 3 — an intentional change re-baselines BOTH in the
// same commit).
func TestCannedMatchesDefault(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "tui", "testdata", "parity", "canned.json"))
	if err != nil {
		t.Fatalf("canned.json: %v", err)
	}
	b, err := LoadBook(raw)
	if err != nil {
		t.Fatalf("LoadBook: %v", err)
	}
	if !reflect.DeepEqual(b, DefaultBook()) {
		t.Fatalf("canned.json drifted from DefaultBook:\n got %+v\nwant %+v", b, DefaultBook())
	}
}

// TestNewPromptAfterToolTurnIsNotToolReply pins the single-turn scope: the
// tool-result follow-up is keyed off the LAST message — a new user prompt
// after a completed tool turn is a fresh turn, not a tool-result follow-up
// (a history-scoped check would answer it with the canned tool reply); the
// mock re-streams the scripted tool call (the first-prompt behavior).
func TestNewPromptAfterToolTurnIsNotToolReply(t *testing.T) {
	body, err := json.Marshal(map[string]any{
		"stream": true,
		"messages": []any{
			map[string]string{"role": "user", "content": "run the parity check"},
			map[string]any{
				"role":    "assistant",
				"content": "",
				"tool_calls": []any{map[string]any{
					"id":       "call_canned1",
					"function": map[string]string{"name": "bash", "arguments": toolCannedArgs},
				}},
			},
			map[string]string{"role": "tool", "tool_call_id": "call_canned1", "content": "parity-ok\n"},
			map[string]string{"role": "user", "content": "something new"},
		},
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	p := postStream(t, toolCanned(), string(body))
	fs := frames(t, p)
	if len(fs) != 5 {
		t.Fatalf("frame count = %d, want 5 (the scripted tool call re-streamed, not the tool reply)", len(fs))
	}
	if fr := fs[3].Choices[0].FinishReason; fr == nil || *fr != "tool_calls" {
		t.Fatalf("finish = %+v, want tool_calls (a new prompt must not be answered with the canned tool reply)", fs[3])
	}
	var joined string
	for _, f := range fs[1 : len(fs)-2] {
		joined += f.Choices[0].Delta.Content
	}
	if joined == toolCannedReply {
		t.Fatalf("a new prompt after a tool turn was answered with the canned tool reply")
	}
}
